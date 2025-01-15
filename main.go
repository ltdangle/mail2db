package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/jhillyerd/enmime"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Email struct {
	Id          uint   `gorm:"primaryKey"`
	Path        string `gorm:"uniqueIndex;not null"`
	From        string `gorm:"column:from_addr;not null"`
	To          string `gorm:"column:to_addr;not null"`
	DeliveredTo string
	Subject     string
	Body        string
	Date        time.Time
	IsSeen      bool `gorm:"default:false"`
	IsReplied   bool `gorm:"default:false"`
	IsFlaggged  bool `gorm:"default:false"`
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
	// Normalize spaces in date string before parsing
	dateStr := strings.Join(strings.Fields(env.GetHeader("Date")), " ")
	date, err := dateparse.ParseAny(dateStr)
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
		Body:       text,
		IsSeen:     fp.HasFlag(FlagSeen),
		IsReplied:  fp.HasFlag(FlagReplied),
		IsFlaggged: fp.HasFlag(FlagFlagged),
	}, nil
}
func initDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("emails.db?_fk=1&cache=shared&mode=rwc"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
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

func deleteEmail(db *gorm.DB, path string) error {
	result := db.Where("path = ?", path).Delete(&Email{})
	return result.Error
}

func getAllEmailPaths(db *gorm.DB) ([]string, error) {
	var paths []string
	result := db.Model(&Email{}).Pluck("path", &paths)
	return paths, result.Error
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

			existing, err := findEmailByPath(db, path)
			if err != nil {
				log.Printf("Error checking for existing email at %s: %v", path, err)
				fmt.Printf("Skipped file: %s\n", path)
				skippedFiles++
				return nil
			}
			if existing != nil {
				// Skip already processed emails
				fmt.Printf("Skipped file (already exists): %s\n", path)
				skippedFiles++
				return nil
			}

			fmt.Printf("Parsing file: %s\n", path)
			email, err := ParseMaildirFile(path)
			if err != nil {
				fmt.Printf("Skipped file (parse error): %s\n", path)
				skippedFiles++
				return nil
			}

			err = saveEmail(db, email)
			if err != nil {
				log.Printf("Error saving email from %s: %v", path, err)
				fmt.Printf("Skipped file (save error): %s\n", path)
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

	// Get all paths from database
	dbPaths, err := getAllEmailPaths(db)
	if err != nil {
		log.Fatal("Error getting database paths:", err)
	}

	// Create a map of filesystem paths for efficient lookup
	fsPathsMap := make(map[string]bool)
	err = filepath.Walk(mailDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fsPathsMap[path] = true
		}
		return nil
	})
	if err != nil {
		log.Fatal("Error walking filesystem:", err)
	}

	// Delete database records for files that no longer exist
	var deletedCount int
	for _, dbPath := range dbPaths {
		if !fsPathsMap[dbPath] {
			if err := deleteEmail(db, dbPath); err != nil {
				log.Printf("Error deleting email record for %s: %v", dbPath, err)
				continue
			}
			deletedCount++
			fmt.Printf("Deleted database record for missing file: %s\n", dbPath)
		}
	}

	// Print statistics
	fmt.Printf("\nProcessing complete:\n")
	fmt.Printf("Total files found: %d\n", totalFiles)
	fmt.Printf("Successfully parsed and saved: %d\n", parsedFiles)
	fmt.Printf("Skipped files: %d\n", skippedFiles)
	fmt.Printf("Deleted records: %d\n", deletedCount)
}
