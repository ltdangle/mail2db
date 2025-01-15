# Maildir Parser

A CLI tool that processes Maildir format email directories and stores email metadata in SQLite database.

## Features
- Parses emails from Maildir format directories
- Extracts email metadata (From, To, Subject, Date, Body)
- Handles Maildir flags (Seen, Replied, Flagged)
- Skips hidden files and directories
- Maintains database consistency by removing records of deleted emails
- Avoids reprocessing already parsed emails

## Usage
```
go run . <maildir_directory>
```

## Database
Data is stored in a SQLite database file named `emails.db` with the following fields:
- Path (unique)
- From address
- To address
- Subject
- Body
- Date
- Flag states (Seen, Replied, Flagged)
