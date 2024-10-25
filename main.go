package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhillyerd/enmime"
	_ "github.com/mattn/go-sqlite3"
)

type Email struct {
	Id int
	// Email's path on the filesystem
	Path        string
	From        string
	To          string
	DeliveredTo string
	Subject     string
	Text        string
	HTML        string
	Date        string

	// Email flags
	IsSeen      bool
	IsImportant bool
	IsAnswered  bool
	IsSelected  bool
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
	date := env.GetHeader("Date")
	from := env.GetHeader("From")
	to := env.GetHeader("To")
	subject := env.GetHeader("Subject")
	text := env.Text

	// Construct and return an Email struct.
	return &Email{
		Path:    path,
		Date:    date,
		From:    from,
		To:      to,
		Subject: subject,
		Text:    text,
	}
}
func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "emails.db")
	if err != nil {
		log.Fatal(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL,
		from_addr TEXT NOT NULL,
		to_addr TEXT NOT NULL,
		delivered_to TEXT,
		subject TEXT,
		text TEXT,
		html TEXT,
		date TEXT,
		is_seen INTEGER DEFAULT 0,
		is_important INTEGER DEFAULT 0,
		is_answered INTEGER DEFAULT 0,
		is_selected INTEGER DEFAULT 0,
		UNIQUE(path)
	);`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func saveEmail(db *sql.DB, email *Email) error {
	query := `
	INSERT INTO emails (
		path, from_addr, to_addr, delivered_to, 
		subject, text, html, date,
		is_seen, is_important, is_answered, is_selected
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, 
		email.Path, email.From, email.To, email.DeliveredTo,
		email.Subject, email.Text, email.HTML, email.Date,
		boolToInt(email.IsSeen), boolToInt(email.IsImportant), 
		boolToInt(email.IsAnswered), boolToInt(email.IsSelected))
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
	defer db.Close()

	err = filepath.Walk(mailDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			email := ParseMaildirFile(path)
			err = saveEmail(db, email)
			if err != nil {
				if strings.Contains(err.Error(), "UNIQUE constraint failed") {
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
