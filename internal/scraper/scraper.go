package scraper

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

//go:embed scrape.py
var scrapePy string

//go:embed fetch.py
var fetchPy string

type ScrapedArticle struct {
	Title         string
	URL           string
	PublishedDate *time.Time
}

type ScrapeError struct {
	Message string
}

func (e ScrapeError) Error() string {
	return e.Message
}

type scrapeResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	PublishedDate *string `json:"published_date"`
}

func ScrapeBlog(blogURL string, selector string, timeout time.Duration) ([]ScrapedArticle, error) {
	tmpFile, err := os.CreateTemp("", "scrape-*.py")
	if err != nil {
		return nil, ScrapeError{Message: fmt.Sprintf("failed to create temp file: %v", err)}
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(scrapePy); err != nil {
		tmpFile.Close()
		return nil, ScrapeError{Message: fmt.Sprintf("failed to write script: %v", err)}
	}
	tmpFile.Close()

	timeoutSecs := strconv.Itoa(int(timeout.Seconds()))
	ctx, cancel := context.WithTimeout(context.Background(), timeout+30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, findPython(), tmpFile.Name(), blogURL, selector, timeoutSecs)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return nil, ScrapeError{Message: fmt.Sprintf("scrape failed: %s", msg)}
	}

	var results []scrapeResult
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil, ScrapeError{Message: fmt.Sprintf("failed to parse scrape output: %v", err)}
	}

	articles := make([]ScrapedArticle, 0, len(results))
	for _, r := range results {
		articles = append(articles, ScrapedArticle{
			Title: r.Title,
			URL:   r.URL,
		})
	}
	return articles, nil
}

func FetchRaw(targetURL string, timeout time.Duration) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "fetch-*.py")
	if err != nil {
		return nil, ScrapeError{Message: fmt.Sprintf("failed to create temp file: %v", err)}
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(fetchPy); err != nil {
		tmpFile.Close()
		return nil, ScrapeError{Message: fmt.Sprintf("failed to write script: %v", err)}
	}
	tmpFile.Close()

	timeoutSecs := strconv.Itoa(int(timeout.Seconds()))
	ctx, cancel := context.WithTimeout(context.Background(), timeout+30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, findPython(), tmpFile.Name(), targetURL, timeoutSecs)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return nil, ScrapeError{Message: fmt.Sprintf("fetch failed: %s", msg)}
	}

	return stdout.Bytes(), nil
}

func findPython() string {
	home, err := os.UserHomeDir()
	if err == nil {
		venvPython := filepath.Join(home, ".blogwatcher", "venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}
	return "python3"
}

func IsScrapeError(err error) bool {
	var scrapeErr ScrapeError
	return errors.As(err, &scrapeErr)
}
