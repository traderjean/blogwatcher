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
3. **Read content** *only when needed* — see the fetch tips below.
4. **Compose the brief.** Cluster by topic (multiple blogs covering the same story collapse into one bullet). Lead with what's actually new, not a per-blog dump. Mention which blog said what.
5. **Offer cleanup.** "Mark these N as read?" → on yes, `mark_read({article_ids: [...]})`.

If the user just wants the queue without summaries, return a short list of titles + URLs and stop. Don't auto-summarize unless they asked.

Brief-generation fetch tips

The killer workflow can blow up two ways: oversized payloads (bulk fetches accumulating into 500K+ char responses) and wasted round trips on JS-rendered sites. Apply these:

- **Title-first.** `list_articles` returns titles. For ~70% of briefs, titles alone are enough to cluster and summarize — fetch bodies only when a title is ambiguous, when the user wants depth, or when two articles look like they cover the same story and you need confirmation.
- **No bulk fetches for full pages.** Scrapling's `bulk_get` / `bulk_fetch` are tuned for parallel API/feed fetches with small bodies. For HTML article bodies, use **single-URL `fetch` calls in a loop, summarize each before fetching the next, then discard.** Don't accumulate raw HTML across many URLs in your context.
- **Cap batches.** If you do call a bulk tool, cap at 5 URLs per call. Larger batches risk overflowing tool-result token limits and triggering "saved to file" fallbacks that are awkward to read back.
- **Skip `get` for SPA hosts.** These sites are React/Next.js shells that return nav-only HTML to plain HTTP fetches. Go straight to Scrapling's `fetch` (Playwright-rendered):
    - `hotelrank.ai`
    - `examine.com`
    - `revinate.com`
    - any site where `list_blogs` shows a `scrape_selector` configured (those were chosen specifically because RSS/static fetch didn't work)
- **Trust 404s.** If a fetch returns 404, the publisher deleted the article. Drop it from the brief, mention briefly. Don't retry, don't escalate to stealthy_fetch — 404 is 404.

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
- **Known limitation:** scans don't honor request cancellation. If the user closes the chat mid-scan, the underlying scrapers run to completion. Prefer scoped scans (single category or single blog) over `scan()` with no args.

Partial-success semantics

- `add_blog` may succeed in creating the blog but fail to assign categories. In that case the result has `category_errors` and `IsError=false` — the blog was created. Tell the user which categories failed and offer to retry just those.
- `mark_read` only flags `IsError=true` if *zero* IDs succeeded. Partial successes return `IsError=false` with an `errors` array listing the failed IDs. Don't auto-retry the whole batch — re-issue only for the IDs in `errors`.

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
