package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemDBSync(t *testing.T) {
	// Setup
	testDir := "test_emails"
	db := initDB()
	
	// Cleanup after test
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	defer os.Remove("emails.db")

	// Step 1: Initial load of all test files
	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// Skip non-email files (like .DS_Store)
			if !strings.Contains(path, "Dangles-MacBook-Pro") {
				return nil
			}
			
			email, err := ParseMaildirFile(path)
			if err != nil {
				t.Logf("Skipping file %s: %v", path, err)
				return nil
			}
			if err := saveEmail(db, email); err != nil {
				t.Logf("Failed to save email %s: %v", path, err)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to load initial test files: %v", err)
	}

	// Step 2: Verify initial count
	var initialCount int64
	db.Model(&Email{}).Count(&initialCount)
	if initialCount == 0 {
		t.Fatal("No emails were loaded into database")
	}

	// Step 3: Temporarily move a test file (simulating deletion)
	testFile := filepath.Join(testDir, "1729845003.21247_1.Dangles-MacBook-Pro,U=11297:2,S")
	tempFile := testFile + ".tmp"
	
	if err := os.Rename(testFile, tempFile); err != nil {
		t.Fatalf("Failed to move test file: %v", err)
	}
	// Ensure we restore the file after test
	defer os.Rename(tempFile, testFile)

	// Step 4: Run the sync process
	dbPaths, err := getAllEmailPaths(db)
	if err != nil {
		t.Fatalf("Failed to get DB paths: %v", err)
	}

	fsPathsMap := make(map[string]bool)
	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fsPathsMap[path] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk filesystem: %v", err)
	}

	for _, dbPath := range dbPaths {
		if !fsPathsMap[dbPath] {
			if err := deleteEmail(db, dbPath); err != nil {
				t.Fatalf("Failed to delete email: %v", err)
			}
		}
	}

	// Step 5: Verify the deletion
	var email Email
	result := db.Where("path = ?", testFile).First(&email)
	if result.Error == nil {
		t.Error("Email record should have been deleted but still exists")
	}

	// Step 6: Verify final count
	var finalCount int64
	db.Model(&Email{}).Count(&finalCount)
	if finalCount != initialCount-1 {
		t.Errorf("Expected count to decrease by 1, got initial: %d, final: %d", initialCount, finalCount)
	}
}
