---
name: blogwatcher-mcp
description: Read and act on the user's tracked blogs and articles via the local blogwatcher SQLite database. Use this when the user asks "what's new in my blogs", wants a daily/weekly digest by category, asks to add or scan blogs, or wants to mark articles read. Pairs with the Scrapling MCP for content reading — blogwatcher gives you the URL queue, Scrapling fetches the page bodies.
homepage: https://github.com/traderjean/blogwatcher
---

# blogwatcher-mcp

Local stdio MCP server that exposes the user's blogwatcher database. Same SQLite file as the `blogwatcher` CLI at `~/.blogwatcher/blogwatcher.db` — changes made via the MCP show up in the CLI immediately and vice versa.

When to use this skill

- The user asks "what's new", "show me unread", "what's in my [category]", "give me a brief".
- The user wants to add a blog, scan, mark articles read, or query categories/blogs.
- The user references their tracked blogs in any way.

Tools

| Tool | Purpose |
|---|---|
| `list_articles` | Filter articles by category, blog_name, unread_only, since_days, limit. Returns id (use with mark_read), title, url, blog name. |
| `list_blogs` | List tracked blogs with feeds, categories, last-scanned. |
| `list_categories` | List categories with blog counts. Confirm exact name before filtering. |
| `mark_read` | Mark articles as read by ID array. |
| `mark_all_read` | Bulk mark by category / blog / uncategorized. |
| `add_blog` | Add a new blog with optional feed_url, scrape_selector, categories. |
| `scan` | Trigger scan (slow: 0.1-7s per blog). |

The killer workflow: daily/weekly brief

This is what blogwatcher MCP enables that the CLI can't do alone. Pattern:

1. **Discover scope.** `list_categories` to confirm the user's category name. If they said "marketing" but the category is "marketing-ops", surface that.
2. **Pull queue.** `list_articles(category=X, unread_only=true, since_days=7, limit=30)`.
3. **Read content.** For each URL in the result, call the Scrapling MCP — `get` first (fast), escalate to `stealthy_fetch` only if you hit a 403 or empty response. Don't headless-browser everything; it's slow.
4. **Compose the brief.** Cluster by topic (multiple blogs covering the same story collapse into one bullet). Lead with what's actually new, not a per-blog dump. Mention which blog said what.
5. **Offer cleanup.** "Mark these N as read?" → on yes, `mark_read({article_ids: [...]})`.

If the user just wants the queue without summaries, return a short list of titles + URLs and stop. Don't auto-summarize unless they asked.

Adding blogs conversationally

When the user says "add the Hugging Face blog to AI", do:
1. `list_categories` to see if `ai` exists (and what spelling — `ai` vs `AI`).
2. `add_blog({name, url, categories: ["ai"]})`. Categories auto-create if missing.
3. **CRITICAL**: if the URL the user gave is itself the feed (ends in /feed, /rss, .xml), pass it as `feed_url` too. Otherwise auto-discovery fires on first scan.

Don't scan immediately after add unless asked — scan is slow.

Scanning

- `scan()` with no args = all blogs. Can take 1-3 minutes for ~50 blogs (most are RSS, fast; a few use stealth scraping which is ~3-7s each).
- Scope when possible: `scan({category: "X"})` is friendlier.
- Report scan results as "N new across M blogs", listing the blogs with new content. Don't dump all per-blog rows unless the user asks.

Output the user already understands

The user runs `blogwatcher scan --category X` and `blogwatcher articles --category X` from the terminal. So they understand the underlying model: blogs → articles, articles flip from unread to read. Don't over-explain.

Handling sources

`list_articles` returns articles regardless of how they were discovered (RSS or stealth-scraped). `list_blogs` shows `feed_url` and `scrape_selector` per blog if you need to know how a blog is sourced. The `scan` results include a `source` field: `rss`, `rss+scrapling` (RSS got blocked, fell back to TLS-impersonating fetch), or `scraper` (no feed, used StealthyFetcher with selector).

Verifying the MCP is connected

```bash
claude mcp list | grep blogwatcher
# Expected: blogwatcher: /opt/homebrew/bin/blogwatcher-mcp - ✓ Connected
```

If broken, re-register:

```bash
claude mcp add blogwatcher --scope user /opt/homebrew/bin/blogwatcher-mcp
```

Claude Desktop reads `~/Library/Application Support/Claude/claude_desktop_config.json` separately — needs full app relaunch (Cmd-Q) after edits.
