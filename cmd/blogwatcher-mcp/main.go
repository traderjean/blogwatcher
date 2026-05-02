// Command blogwatcher-mcp exposes the local blogwatcher database to MCP
// clients (Claude Desktop, Claude Code) over stdio. It reads/writes the same
// SQLite file as the `blogwatcher` CLI at ~/.blogwatcher/blogwatcher.db.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/traderjean/blogwatcher/internal/controller"
	"github.com/traderjean/blogwatcher/internal/model"
	"github.com/traderjean/blogwatcher/internal/scanner"
	"github.com/traderjean/blogwatcher/internal/storage"
)

const scanWorkers = 4

func main() {
	if err := run(); err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	path, err := storage.DefaultDBPath()
	if err != nil {
		return fmt.Errorf("db path: %w", err)
	}
	db, err := storage.OpenDatabase(path)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	srv := mcp.NewServer(&mcp.Implementation{Name: "blogwatcher", Version: "0.1.0"}, nil)
	registerTools(srv, db)

	// Honor SIGTERM/SIGINT for graceful shutdown so the deferred db.Close
	// runs. Claude Desktop usually SIGKILLs at session end (defer skipped),
	// but WAL mode means no data loss; this handles cooperative shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		// Cooperative shutdown via SIGTERM/SIGINT cancels ctx, which the
		// SDK surfaces as context.Canceled. That's not a fatal error.
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("mcp run: %w", err)
	}
	return nil
}

func registerTools(srv *mcp.Server, db *storage.Database) {
	registerListArticles(srv, db)
	registerListBlogs(srv, db)
	registerListCategories(srv, db)
	registerMarkRead(srv, db)
	registerMarkAllRead(srv, db)
	registerAddBlog(srv, db)
	registerScan(srv, db)
}

type ListArticlesArgs struct {
	Category      string `json:"category,omitempty" jsonschema:"filter by category name"`
	Uncategorized bool   `json:"uncategorized,omitempty" jsonschema:"limit to blogs without any category"`
	BlogName      string `json:"blog_name,omitempty" jsonschema:"exact blog name to filter by"`
	UnreadOnly    bool   `json:"unread_only,omitempty" jsonschema:"return only unread articles"`
	SinceDays     int    `json:"since_days,omitempty" jsonschema:"only articles whose published or discovered date is within last N days"`
	Limit         int    `json:"limit,omitempty" jsonschema:"max articles to return; default 100"`
}

func registerListArticles(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_articles",
		Description: "List tracked articles. Use unread_only=true to find new content. Filter by category, blog_name, " +
			"or since_days to narrow scope. Returns id (use with mark_read), title, url, blog name, and dates.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListArticlesArgs) (*mcp.CallToolResult, any, error) {
		showAll := !args.UnreadOnly
		articles, blogNames, err := controller.GetArticles(db, showAll, args.BlogName, args.Category, args.Uncategorized)
		if err != nil {
			return nil, nil, err
		}

		if args.SinceDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -args.SinceDays)
			filtered := articles[:0]
			for _, a := range articles {
				d := a.PublishedDate
				if d == nil {
					d = a.DiscoveredDate
				}
				if d != nil && d.After(cutoff) {
					filtered = append(filtered, a)
				}
			}
			articles = filtered
		}

		limit := args.Limit
		if limit <= 0 {
			limit = 100
		}
		truncated := false
		if len(articles) > limit {
			articles = articles[:limit]
			truncated = true
		}

		type out struct {
			ID         int64  `json:"id"`
			Title      string `json:"title"`
			URL        string `json:"url"`
			Blog       string `json:"blog"`
			Published  string `json:"published_date,omitempty"`
			Discovered string `json:"discovered_date,omitempty"`
			Read       bool   `json:"read"`
		}
		result := make([]out, 0, len(articles))
		for _, a := range articles {
			o := out{ID: a.ID, Title: a.Title, URL: a.URL, Blog: blogNames[a.BlogID], Read: a.IsRead}
			if a.PublishedDate != nil {
				o.Published = a.PublishedDate.Format(time.RFC3339)
			}
			if a.DiscoveredDate != nil {
				o.Discovered = a.DiscoveredDate.Format(time.RFC3339)
			}
			result = append(result, o)
		}
		return jsonResult(map[string]any{
			"count":     len(result),
			"truncated": truncated,
			"articles":  result,
		})
	})
}

type ListBlogsArgs struct {
	Category      string `json:"category,omitempty"`
	Uncategorized bool   `json:"uncategorized,omitempty"`
}

func registerListBlogs(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_blogs",
		Description: "List tracked blogs with feed URL, scrape selector, last-scanned time, and assigned categories.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListBlogsArgs) (*mcp.CallToolResult, any, error) {
		var blogs []model.Blog
		var err error
		switch {
		case args.Uncategorized:
			blogs, err = db.ListUncategorizedBlogs()
		case args.Category != "":
			cat, cerr := db.GetCategoryByName(args.Category)
			if cerr != nil {
				return nil, nil, cerr
			}
			if cat == nil {
				return nil, nil, fmt.Errorf("unknown category: %s", args.Category)
			}
			blogs, err = db.ListBlogsByCategory(cat.ID)
		default:
			blogs, err = db.ListBlogs()
		}
		if err != nil {
			return nil, nil, err
		}

		type out struct {
			ID             int64    `json:"id"`
			Name           string   `json:"name"`
			URL            string   `json:"url"`
			FeedURL        string   `json:"feed_url,omitempty"`
			ScrapeSelector string   `json:"scrape_selector,omitempty"`
			LastScanned    string   `json:"last_scanned,omitempty"`
			Categories     []string `json:"categories,omitempty"`
		}
		result := make([]out, 0, len(blogs))
		for _, b := range blogs {
			cats, cerr := db.GetBlogCategories(b.ID)
			if cerr != nil {
				return nil, nil, fmt.Errorf("load categories for blog %d: %w", b.ID, cerr)
			}
			catNames := make([]string, 0, len(cats))
			for _, c := range cats {
				catNames = append(catNames, c.Name)
			}
			o := out{ID: b.ID, Name: b.Name, URL: b.URL, FeedURL: b.FeedURL, ScrapeSelector: b.ScrapeSelector, Categories: catNames}
			if b.LastScanned != nil {
				o.LastScanned = b.LastScanned.Format(time.RFC3339)
			}
			result = append(result, o)
		}
		return jsonResult(map[string]any{"count": len(result), "blogs": result})
	})
}

func registerListCategories(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_categories",
		Description: "List all categories with blog counts. Use this before filtering by category to confirm the exact name.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		cats, err := db.ListCategories()
		if err != nil {
			return nil, nil, err
		}
		type out struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Blogs int    `json:"blogs"`
		}
		result := make([]out, 0, len(cats))
		for _, c := range cats {
			blogs, berr := db.ListBlogsByCategory(c.ID)
			if berr != nil {
				return nil, nil, fmt.Errorf("count blogs for category %d: %w", c.ID, berr)
			}
			result = append(result, out{ID: c.ID, Name: c.Name, Blogs: len(blogs)})
		}
		return jsonResult(map[string]any{"count": len(result), "categories": result})
	})
}

type MarkReadArgs struct {
	ArticleIDs []int64 `json:"article_ids" jsonschema:"article IDs to mark as read"`
}

func registerMarkRead(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "mark_read",
		Description: "Mark one or more articles as read by ID. Pass IDs returned from list_articles.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args MarkReadArgs) (*mcp.CallToolResult, any, error) {
		marked := 0
		var errs []string
		for _, id := range args.ArticleIDs {
			if _, err := controller.MarkArticleRead(db, id); err != nil {
				errs = append(errs, fmt.Sprintf("id=%d: %v", id, err))
				continue
			}
			marked++
		}
		out := map[string]any{"marked": marked}
		if len(errs) > 0 {
			out["errors"] = errs
		}
		// Only flag IsError when *nothing* succeeded. Partial success is
		// surfaced via the errors array — flagging the whole op as failed
		// would mislead clients about articles that did get marked.
		return jsonResultIsError(out, marked == 0 && len(errs) > 0)
	})
}

type MarkAllReadArgs struct {
	Category      string `json:"category,omitempty"`
	Uncategorized bool   `json:"uncategorized,omitempty"`
	BlogName      string `json:"blog_name,omitempty"`
}

func registerMarkAllRead(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "mark_all_read",
		Description: "Mark all currently unread articles as read, optionally scoped by category, blog_name, or uncategorized.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args MarkAllReadArgs) (*mcp.CallToolResult, any, error) {
		marked, err := controller.MarkAllArticlesRead(db, args.BlogName, args.Category, args.Uncategorized)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(map[string]any{"marked": len(marked)})
	})
}

type AddBlogArgs struct {
	Name           string   `json:"name" jsonschema:"display name for the blog"`
	URL            string   `json:"url" jsonschema:"homepage URL or feed URL"`
	FeedURL        string   `json:"feed_url,omitempty" jsonschema:"explicit feed URL; required when URL itself IS the feed"`
	ScrapeSelector string   `json:"scrape_selector,omitempty" jsonschema:"CSS selector for HTML scraping fallback"`
	Categories     []string `json:"categories,omitempty" jsonschema:"category names to assign; auto-created if missing"`
}

func registerAddBlog(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "add_blog",
		Description: "Add a new blog to track. If url IS the feed (ends in /feed, /rss, .xml), pass the same URL as feed_url too. " +
			"Categories are created on-the-fly if they don't exist.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args AddBlogArgs) (*mcp.CallToolResult, any, error) {
		if args.Name == "" || args.URL == "" {
			return nil, nil, fmt.Errorf("name and url are required")
		}
		blog, err := controller.AddBlog(db, args.Name, args.URL, args.FeedURL, args.ScrapeSelector)
		if err != nil {
			return nil, nil, err
		}
		var assignErrors []string
		for _, c := range args.Categories {
			// Auto-create the category if missing — AssignBlogToCategory
			// alone returns CategoryNotFoundError for unknown names.
			if _, err := controller.GetOrCreateCategory(db, c); err != nil {
				assignErrors = append(assignErrors, fmt.Sprintf("%s: ensure: %v", c, err))
				continue
			}
			if err := controller.AssignBlogToCategory(db, blog.Name, c); err != nil {
				assignErrors = append(assignErrors, fmt.Sprintf("%s: assign: %v", c, err))
			}
		}
		out := map[string]any{"id": blog.ID, "name": blog.Name, "url": blog.URL}
		if len(assignErrors) > 0 {
			out["category_errors"] = assignErrors
		}
		// Blog row was created successfully; category_errors signals
		// partial work. Flagging IsError here would tell the client the
		// blog wasn't created and prompt a duplicate-add retry.
		return jsonResult(out)
	})
}

type ScanArgs struct {
	Category      string `json:"category,omitempty"`
	Uncategorized bool   `json:"uncategorized,omitempty"`
	BlogName      string `json:"blog_name,omitempty" jsonschema:"single blog name to scan"`
}

func registerScan(srv *mcp.Server, db *storage.Database) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "scan",
		Description: "Scan blogs for new articles. Defaults to all blogs. Can be slow (each blog ~0.1-7s). " +
			"Returns per-blog results with new article counts and any errors.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ScanArgs) (*mcp.CallToolResult, any, error) {
		var results []scanner.ScanResult
		var err error
		switch {
		case args.BlogName != "":
			r, e := scanner.ScanBlogByName(db, args.BlogName)
			if e != nil {
				return nil, nil, e
			}
			if r == nil {
				return nil, nil, fmt.Errorf("blog not found: %s", args.BlogName)
			}
			results = []scanner.ScanResult{*r}
		case args.Uncategorized:
			results, err = scanner.ScanUncategorizedBlogs(db, scanWorkers)
		case args.Category != "":
			results, err = scanner.ScanBlogsByCategory(db, args.Category, scanWorkers)
		default:
			results, err = scanner.ScanAllBlogs(db, scanWorkers)
		}
		if err != nil {
			return nil, nil, err
		}

		type out struct {
			Blog   string `json:"blog"`
			Source string `json:"source"`
			Found  int    `json:"found"`
			New    int    `json:"new"`
			Error  string `json:"error,omitempty"`
		}
		rs := make([]out, 0, len(results))
		totalNew := 0
		for _, r := range results {
			rs = append(rs, out{Blog: r.BlogName, Source: r.Source, Found: r.TotalFound, New: r.NewArticles, Error: r.Error})
			totalNew += r.NewArticles
		}
		return jsonResult(map[string]any{"scanned": len(rs), "new_articles": totalNew, "results": rs})
	})
}

func jsonResult(payload any) (*mcp.CallToolResult, any, error) {
	return jsonResultIsError(payload, false)
}

func jsonResultIsError(payload any, isError bool) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		IsError: isError,
	}, nil, nil
}
