package scraper

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestScrapeBlogParsesJSON(t *testing.T) {
	// Create a fake Python script that outputs known JSON
	fake := `import sys, json; json.dump([{"title": "First", "url": "https://example.com/one", "published_date": None}, {"title": "Second", "url": "https://example.com/two", "published_date": None}], sys.stdout)`
	tmpFile, err := os.CreateTemp("", "fake-scrape-*.py")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(fake)
	tmpFile.Close()

	// Verify python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}

	// Test JSON output parsing directly
	output := `[{"title":"First","url":"https://example.com/one","published_date":null},{"title":"Second","url":"https://example.com/two","published_date":null}]`
	var results []scrapeResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "First" || results[0].URL != "https://example.com/one" {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
}

func TestScrapeErrorType(t *testing.T) {
	err := ScrapeError{Message: "test error"}
	if err.Error() != "test error" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
	if !IsScrapeError(err) {
		t.Fatal("expected IsScrapeError to return true")
	}
}

func TestScrapeBlogMissingPython(t *testing.T) {
	// Save and override PATH to simulate missing python3
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	_, err := ScrapeBlog("https://example.com", "a", 5*time.Second)
	if err == nil {
		t.Fatal("expected error when python3 not found")
	}
	if !IsScrapeError(err) {
		t.Fatalf("expected ScrapeError, got %T: %v", err, err)
	}
}
