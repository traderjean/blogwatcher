---
name: blogwatcher
description: Monitor blogs and RSS/Atom feeds for updates using the blogwatcher CLI. Supports categories for organizing blogs by topic.
homepage: https://github.com/traderjean/blogwatcher
metadata:
  {
    "openclaw":
      {
        "emoji": "📰",
        "requires": { "bins": ["blogwatcher"] },
        "install":
          [
            {
              "id": "go",
              "kind": "go",
              "module": "github.com/traderjean/blogwatcher/cmd/blogwatcher@latest",
              "bins": ["blogwatcher"],
              "label": "Install blogwatcher (go)",
            },
          ],
      },
  }
---

# blogwatcher

Track blog and RSS/Atom feed updates with the `blogwatcher` CLI. Organize blogs into categories for filtered scanning and reading.

Install

- Go: `go install github.com/traderjean/blogwatcher/cmd/blogwatcher@latest`

Quick start

- `blogwatcher --help`

Common commands

- Add a blog: `blogwatcher add "My Blog" https://example.com`
- Add with categories: `blogwatcher add "My Blog" https://example.com --category seo --category marketing`
- List blogs: `blogwatcher blogs`
- List by category: `blogwatcher blogs --category seo`
- List uncategorized: `blogwatcher blogs --uncategorized`
- Scan for updates: `blogwatcher scan`
- Scan a category: `blogwatcher scan --category seo`
- List articles: `blogwatcher articles`
- List by category: `blogwatcher articles --category seo`
- Mark an article read: `blogwatcher read 1`
- Mark all articles read: `blogwatcher read-all`
- Mark category read: `blogwatcher read-all --category seo --yes`
- Remove a blog: `blogwatcher remove "My Blog"`

Category management

- Create: `blogwatcher category add seo`
- List: `blogwatcher category list`
- Assign: `blogwatcher category assign "My Blog" seo`
- Unassign: `blogwatcher category unassign "My Blog" seo`
- Remove (keeps blogs): `blogwatcher category remove seo`

Example output

```
$ blogwatcher blogs --category seo
Tracked blogs (2):

  Moz Blog
    URL: https://moz.com/blog/feed
    Feed: https://moz.com/feed
    Categories: seo
    Last scanned: 2026-03-20 18:00

  Skift
    URL: https://skift.com/feed/
    Feed: https://skift.com/feed
    Categories: hospitality, seo
    Last scanned: 2026-03-20 18:00
```

```
$ blogwatcher scan --category seo
Scanning 2 blog(s)...

  Moz Blog
    Source: RSS | Found: 10 | New: 2
  Skift
    Source: RSS | Found: 10 | New: 0

Found 2 new article(s) total!
```

Notes

- Use `blogwatcher <command> --help` to discover flags and options.
- `--category` and `--uncategorized` flags are mutually exclusive.
- Blogs can belong to multiple categories.
- All commands work unchanged without `--category` (backwards compatible).
