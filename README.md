# BlogWatcher

> Fork of [Hyaxia/blogwatcher](https://github.com/Hyaxia/blogwatcher) with added category support for organizing blogs by topic.

A Go CLI tool to track blog articles, detect new posts, and manage read/unread status. Supports both RSS/Atom feeds and HTML scraping as fallback.

## Alternatives

If you're looking for RSS in OpenClaw, [openclaw-skill-rss-digest](https://github.com/manthis/openclaw-skill-rss-digest) is another option. We preferred forking blogwatcher for more control over scraping, categories, and CLI behavior.

## What's New in This Fork

- **Categories** — Organize blogs into categories (many-to-many). Filter scans, articles, and listings by category. Existing commands work unchanged when no category is specified.

## Migrating from OpenClaw's Built-in Blogwatcher

OpenClaw ships with a bundled blogwatcher skill that points to the original repo. To switch to this fork:

1. **Install the fork binary:**
   ```bash
   go install github.com/traderjean/blogwatcher/cmd/blogwatcher@latest
   # Or build from source:
   git clone https://github.com/traderjean/blogwatcher.git
   cd blogwatcher
   go build -o /opt/homebrew/bin/blogwatcher ./cmd/blogwatcher
   ```

2. **Copy the OpenClaw skill override:**
   ```bash
   mkdir -p ~/.openclaw/skills/blogwatcher
   cp openclaw/SKILL.md ~/.openclaw/skills/blogwatcher/SKILL.md
   ```

3. **Disable the bundled skill** in `~/.openclaw/openclaw.json`:
   ```json
   {
     "skills": {
       "entries": {
         "blogwatcher": { "enabled": false }
       }
     }
   }
   ```

Your existing database at `~/.blogwatcher/blogwatcher.db` is upgraded automatically on first run (no migration needed).

## Features

-   **Categories** - Organize blogs by topic and filter by category
-   **Dual Source Support** - Tries RSS feeds first, falls back to HTML scraping
-   **Automatic Feed Discovery** - Detects RSS/Atom URLs from blog homepages
-   **Read/Unread Management** - Track which articles you've read
-   **Blog Filtering** - View articles from specific blogs or categories
-   **Duplicate Prevention** - Never tracks the same article twice
-   **Colored CLI Output** - User-friendly terminal interface

## Installation

```bash
# Install from this fork
go install github.com/traderjean/blogwatcher/cmd/blogwatcher@latest

# Or build locally
git clone https://github.com/traderjean/blogwatcher.git
cd blogwatcher
go build -o /opt/homebrew/bin/blogwatcher ./cmd/blogwatcher
```

## Usage

### Adding Blogs

```bash
# Add a blog (auto-discovers RSS feed)
blogwatcher add "My Favorite Blog" https://example.com/blog

# Add with explicit feed URL
blogwatcher add "Tech Blog" https://techblog.com --feed-url https://techblog.com/rss.xml

# Add with HTML scraping selector (for blogs without feeds)
blogwatcher add "No-RSS Blog" https://norss.com --scrape-selector "article h2 a"

# Add and assign to categories (auto-creates if needed)
blogwatcher add "SEO Blog" https://seoblog.com --category seo --category marketing
```

### Managing Blogs

```bash
# List all tracked blogs
blogwatcher blogs

# Filter by category
blogwatcher blogs --category seo

# Show blogs with no categories assigned
blogwatcher blogs --uncategorized

# Remove a blog (and all its articles)
blogwatcher remove "My Favorite Blog"
```

### Categories

Blogs can belong to multiple categories. All commands that list or filter blogs/articles accept `--category` and `--uncategorized` flags.

```bash
# Create a category
blogwatcher category add seo

# List all categories
blogwatcher category list

# Assign a blog to a category
blogwatcher category assign "Tech Blog" seo

# Assign to multiple categories
blogwatcher category assign "Tech Blog" marketing

# Remove a blog from a category
blogwatcher category unassign "Tech Blog" marketing

# Remove a category (blogs are kept)
blogwatcher category remove seo
```

### Scanning for New Articles

```bash
# Scan all blogs for new articles
blogwatcher scan

# Scan a specific blog
blogwatcher scan "Tech Blog"

# Scan only blogs in a category
blogwatcher scan --category seo

# Scan only uncategorized blogs
blogwatcher scan --uncategorized
```

### Viewing Articles

```bash
# List unread articles
blogwatcher articles

# List all articles (including read)
blogwatcher articles --all

# List articles from a specific blog
blogwatcher articles --blog "Tech Blog"

# List articles from a category
blogwatcher articles --category seo

# List articles from uncategorized blogs
blogwatcher articles --uncategorized
```

### Managing Read Status

```bash
# Mark an article as read (use article ID from articles list)
blogwatcher read 42

# Mark an article as unread
blogwatcher unread 42

# Mark all unread articles as read
blogwatcher read-all

# Mark all unread articles as read for a blog (skip prompt)
blogwatcher read-all --blog "Tech Blog" --yes

# Mark all unread articles in a category as read
blogwatcher read-all --category seo --yes
```

## How It Works

### Scanning Process

1. For each tracked blog, BlogWatcher first attempts to parse the RSS/Atom feed
2. If no feed URL is configured, it tries to auto-discover one from the blog homepage
3. If RSS parsing fails and a `scrape_selector` is configured, it falls back to HTML scraping
4. New articles are saved to the database as unread
5. Already-tracked articles are skipped

### Feed Auto-Discovery

BlogWatcher searches for feeds in two ways:

-   Looking for `<link rel="alternate">` tags with RSS/Atom types
-   Checking common feed paths: `/feed`, `/rss`, `/feed.xml`, `/atom.xml`, etc.

### HTML Scraping

When RSS isn't available, provide a CSS selector that matches article links:

```bash
# Example selectors
--scrape-selector "article h2 a"      # Links inside article h2 tags
--scrape-selector ".post-title a"     # Links with post-title class
--scrape-selector "#blog-posts a"     # Links inside blog-posts ID
```

## Database

BlogWatcher stores data in SQLite at `~/.blogwatcher/blogwatcher.db`:

-   **blogs** - Tracked blogs (name, URL, feed URL, scrape selector)
-   **articles** - Discovered articles (title, URL, dates, read status)
-   **categories** - Blog categories (name)
-   **blog_categories** - Many-to-many mapping between blogs and categories

Existing databases are upgraded automatically — the new tables are added on first run with no migration needed.

## Development

### Requirements

-   Go 1.24+

### Running Tests

```bash
go test ./...
```

### Publishing

In addition to publishing to main, a new tag should be published so homebrew will get the updated version:
```
git tag vX.Y.Z
git push origin vX.Y.Z
```

## TODO

- [ ] Replace goquery HTML scraping with [Scrapling](https://github.com/D4Vinci/Scrapling) for adaptive scraping, anti-bot bypass, and smarter element tracking

## License

MIT
