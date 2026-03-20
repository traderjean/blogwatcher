# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BlogWatcher is a Go CLI tool to track blog articles, detect new posts, and manage read/unread status. It supports both RSS/Atom feeds and HTML scraping as fallback. Fork of [Hyaxia/blogwatcher](https://github.com/Hyaxia/blogwatcher) with added category support.

## Commands

```bash
# Run tests
go test ./...

# Running the project
go run ./cmd/blogwatcher ...
```

## Architecture

### Database
SQLite database stored at `~/.blogwatcher/blogwatcher.db` with four tables:
- `blogs` - Tracked blogs (name, url, feed_url, scrape_selector, last_scanned)
- `articles` - Discovered articles (blog_id, title, url, published_date, discovered_date, is_read)
- `categories` - Blog categories (name, unique)
- `blog_categories` - Many-to-many junction table (blog_id, category_id)

Schema uses `CREATE TABLE IF NOT EXISTS` so existing databases are upgraded automatically.

### Category System
- Blogs can belong to multiple categories (many-to-many via `blog_categories`)
- All listing/filtering commands accept `--category` and `--uncategorized` flags
- When no category flag is passed, commands behave as before (no breaking changes)
- Category CRUD is in `internal/controller` (`AddCategory`, `RemoveCategory`, `AssignBlogToCategory`, etc.)
- Removing a category does NOT delete blogs — only the junction rows are removed

## Tech Stack
- Go 1.24+
- SQLite (modernc.org/sqlite)
- gofeed (RSS/Atom)
- goquery + net/http (HTML scraping)
- cobra (CLI)
