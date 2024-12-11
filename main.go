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

// ParseMaildirFile parses the maildir file provided, and returns a pointer to the resulting Email struct.
// It takes a string with the path to the maildir file as its only argument, and panics if an error occurs while reading the file.
func ParseMaildirFile(path string) *Email {
	// Read the file.
	raw_msg_byte, err := os.ReadFile(path)
	if err != nil {
		// If an error occurs, panic and print the error message.
		panic(err)
	}
	raw_msg := string(raw_msg_byte)

	// Parse the file using enmime.
	env, _ := enmime.ReadEnvelope(strings.NewReader(raw_msg))

	// Extract the email headers and body.
	date, _ := time.Parse(time.RFC1123Z, env.GetHeader("Date"))
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
	}
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

	err = filepath.Walk(mailDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			email := ParseMaildirFile(path)
			err = saveEmail(db, email)
			if err != nil {
				if err.Error() == "UNIQUE constraint failed: emails.path" {
					// Skip already processed emails silently
					return nil
				}
				log.Printf("Error saving email from %s: %v", path, err)
				return nil
			}

			fmt.Printf("Processed email from: %s\n", email.From)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
