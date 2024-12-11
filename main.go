package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jhillyerd/enmime"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Email struct {
	Id          uint      `gorm:"primaryKey"`
	Path        string    `gorm:"uniqueIndex;not null"`
	From        string    `gorm:"column:from_addr;not null"`
	To          string    `gorm:"column:to_addr;not null"`
	DeliveredTo string
	Subject     string
	Text        string
	HTML        string
	Date        time.Time
	IsSeen      bool      `gorm:"default:false"`
	IsReplied   bool      `gorm:"default:false"`
	IsFlaggged  bool      `gorm:"default:false"`
}

// ParseMaildirFile parses the maildir file provided, and returns a pointer to the resulting Email struct and any error.
func ParseMaildirFile(path string) (*Email, error) {
	// Read the file.
	raw_msg_byte, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	raw_msg := string(raw_msg_byte)

	// Parse the file using enmime.
	env, err := enmime.ReadEnvelope(strings.NewReader(raw_msg))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Extract the email headers and body.
	date, err := time.Parse(time.RFC1123Z, env.GetHeader("Date"))
	if err != nil {
		// If date parsing fails, use current time as fallback
		date = time.Now()
	}
	
	from := env.GetHeader("From")
	to := env.GetHeader("To")
	subject := env.GetHeader("Subject")
	text := env.Text

	// Parse flags from filename
	fp := NewFlagParser(path)

	// Construct and return an Email struct.
	return &Email{
		Path:       path,
		Date:       date,
		From:       from,
		To:         to,
		Subject:    subject,
		Text:       text,
		IsSeen:     fp.HasFlag(FlagSeen),
		IsReplied:  fp.HasFlag(FlagReplied),
		IsFlaggged: fp.HasFlag(FlagFlagged),
	}, nil
}
func initDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Auto migrate the schema
	err = db.AutoMigrate(&Email{})
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func saveEmail(db *gorm.DB, email *Email) error {
	result := db.Create(email)
	return result.Error
}

func findEmailByPath(db *gorm.DB, path string) (*Email, error) {
    var email Email
    result := db.Where("path = ?", path).First(&email)
    if result.Error != nil {
        if result.Error == gorm.ErrRecordNotFound {
            return nil, nil
        }
        return nil, result.Error
    }
    return &email, nil
}

func newEmails(repoPath string) ([]string, error) {
	repoPath = strings.Trim(repoPath, " ")
	cmd := exec.Command("git", "-C", repoPath, "ls-files", "--other")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Split output into lines and construct full paths
	var paths []string
	for _, file := range strings.Split(string(output), "\n") {
		if file != "" {
			paths = append(paths, file)
		}
	}

	return paths, nil
}

func deletedEmails(repoPath string) ([]string, error) {
	repoPath = strings.Trim(repoPath, " ")
	cmd := exec.Command("git", "-C", repoPath, "ls-files", "--deleted")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Split output into lines and construct full paths
	var paths []string
	for _, file := range strings.Split(string(output), "\n") {
		if file != "" {
			paths = append(paths, file)
		}
	}

	return paths, nil
}


func main() {
    if len(os.Args) != 2 {
        fmt.Println("Usage: go run . <imap_mail_directory>")
        os.Exit(1)
    }

    mailDir := os.Args[1]

    // Check if directory exists
    info, err := os.Stat(mailDir)
    if err != nil {
        log.Fatalf("Error accessing directory: %v", err)
    }
    if !info.IsDir() {
        log.Fatalf("%s is not a directory", mailDir)
    }
    db := initDB()

    // Add counters
    var totalFiles, parsedFiles, skippedFiles int

    err = filepath.Walk(mailDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if !info.IsDir() {
            totalFiles++
            fmt.Printf("Parsing file: %s\n", path)
            email, err := ParseMaildirFile(path)
            if err != nil {
                log.Printf("Skipping file %s: %v", path, err)
                skippedFiles++
                return nil
            }

            existing, err := findEmailByPath(db, path)
            if err != nil {
                log.Printf("Error checking for existing email at %s: %v", path, err)
                skippedFiles++
                return nil
            }
            if existing != nil {
                // Skip already processed emails silently
                skippedFiles++
                return nil
            }

            err = saveEmail(db, email)
            if err != nil {
                log.Printf("Error saving email from %s: %v", path, err)
                skippedFiles++
                return nil
            }

            parsedFiles++
            fmt.Printf("Processed email from: %s\n", email.From)
        }

        return nil
    })

    if err != nil {
        log.Fatal(err)
    }

    // Print statistics
    fmt.Printf("\nProcessing complete:\n")
    fmt.Printf("Total files found: %d\n", totalFiles)
    fmt.Printf("Successfully parsed and saved: %d\n", parsedFiles)
    fmt.Printf("Skipped files: %d\n", skippedFiles)
}
