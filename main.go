package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Email struct {
	From    string
	To      string
	Subject string
	Body    string
}

func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "emails.db")
	if err != nil {
		log.Fatal(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_addr TEXT,
		to_addr TEXT,
		subject TEXT,
		body TEXT
	);`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func parseEmailFile(path string) (*Email, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	email := &Email{}
	
	inBody := false
	bodyLines := []string{}

	for _, line := range lines {
		if inBody {
			bodyLines = append(bodyLines, line)
			continue
		}

		if line == "" {
			inBody = true
			continue
		}

		if strings.HasPrefix(line, "From: ") {
			email.From = strings.TrimPrefix(line, "From: ")
		} else if strings.HasPrefix(line, "To: ") {
			email.To = strings.TrimPrefix(line, "To: ")
		} else if strings.HasPrefix(line, "Subject: ") {
			email.Subject = strings.TrimPrefix(line, "Subject: ")
		}
	}

	email.Body = strings.Join(bodyLines, "\n")
	return email, nil
}

func saveEmail(db *sql.DB, email *Email) error {
	query := `
	INSERT INTO emails (from_addr, to_addr, subject, body)
	VALUES (?, ?, ?, ?)`

	_, err := db.Exec(query, email.From, email.To, email.Subject, email.Body)
	return err
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
			email, err := parseEmailFile(path)
			if err != nil {
				log.Printf("Error parsing %s: %v", path, err)
				return nil
			}

			err = saveEmail(db, email)
			if err != nil {
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
