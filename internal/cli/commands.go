package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Hyaxia/blogwatcher/internal/controller"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/scanner"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

func newAddCommand() *cobra.Command {
	var feedURL string
	var scrapeSelector string
	var categories []string

	cmd := &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a new blog to track.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			url := args[1]
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			blog, err := controller.AddBlog(db, name, url, feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			for _, catName := range categories {
				cat, err := controller.GetOrCreateCategory(db, catName)
				if err != nil {
					printError(err)
					return markError(err)
				}
				if err := db.AssignBlogCategory(blog.ID, cat.ID); err != nil {
					printError(err)
					return markError(err)
				}
			}
			color.New(color.FgGreen).Printf("Added blog '%s'\n", name)
			if len(categories) > 0 {
				fmt.Printf("  Categories: %s\n", strings.Join(categories, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "RSS/Atom feed URL (auto-discovered if not provided)")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "CSS selector for HTML scraping fallback")
	cmd.Flags().StringSliceVarP(&categories, "category", "c", nil, "Assign to category (repeatable, auto-creates if needed)")
	return cmd
}

func newRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a blog from tracking.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove blog '%s' and all its articles?", name))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.RemoveBlog(db, name); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newBlogsCommand() *cobra.Command {
	var category string
	var uncategorized bool

	cmd := &cobra.Command{
		Use:   "blogs",
		Short: "List all tracked blogs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if category != "" && uncategorized {
				return fmt.Errorf("--category and --uncategorized are mutually exclusive")
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			var blogs []model.Blog
			if uncategorized {
				blogs, err = db.ListUncategorizedBlogs()
			} else if category != "" {
				cat, catErr := db.GetCategoryByName(category)
				if catErr != nil {
					return catErr
				}
				if cat == nil {
					printError(controller.CategoryNotFoundError{Name: category})
					return markError(controller.CategoryNotFoundError{Name: category})
				}
				blogs, err = db.ListBlogsByCategory(cat.ID)
			} else {
				blogs, err = db.ListBlogs()
			}
			if err != nil {
				return err
			}
			if len(blogs) == 0 {
				fmt.Println("No blogs tracked yet. Use 'blogwatcher add' to add one.")
				return nil
			}
			color.New(color.FgCyan, color.Bold).Printf("Tracked blogs (%d):\n\n", len(blogs))
			for _, blog := range blogs {
				cats, _ := db.GetBlogCategories(blog.ID)
				color.New(color.FgWhite, color.Bold).Printf("  %s\n", blog.Name)
				fmt.Printf("    URL: %s\n", blog.URL)
				if blog.FeedURL != "" {
					fmt.Printf("    Feed: %s\n", blog.FeedURL)
				}
				if blog.ScrapeSelector != "" {
					fmt.Printf("    Selector: %s\n", blog.ScrapeSelector)
				}
				if len(cats) > 0 {
					names := make([]string, len(cats))
					for i, c := range cats {
						names[i] = c.Name
					}
					fmt.Printf("    Categories: %s\n", strings.Join(names, ", "))
				}
				if blog.LastScanned != nil {
					fmt.Printf("    Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
				}
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by category")
	cmd.Flags().BoolVar(&uncategorized, "uncategorized", false, "Show only blogs with no categories")
	return cmd
}

func newScanCommand() *cobra.Command {
	var silent bool
	var workers int
	var category string
	var uncategorized bool

	cmd := &cobra.Command{
		Use:   "scan [blog_name]",
		Short: "Scan blogs for new articles.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if category != "" && uncategorized {
				return fmt.Errorf("--category and --uncategorized are mutually exclusive")
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 1 {
				result, err := scanner.ScanBlogByName(db, args[0])
				if err != nil {
					return err
				}
				if result == nil {
					err := fmt.Errorf("Blog '%s' not found", args[0])
					printError(err)
					return markError(err)
				}
				if !silent {
					printScanResult(*result)
				}
			} else {
				var blogs []model.Blog
				if category != "" {
					cat, catErr := db.GetCategoryByName(category)
					if catErr != nil {
						return catErr
					}
					if cat == nil {
						printError(fmt.Errorf("Category '%s' not found", category))
						return markError(fmt.Errorf("category '%s' not found", category))
					}
					blogs, err = db.ListBlogsByCategory(cat.ID)
				} else if uncategorized {
					blogs, err = db.ListUncategorizedBlogs()
				} else {
					blogs, err = db.ListBlogs()
				}
				if err != nil {
					return err
				}
				if len(blogs) == 0 {
					fmt.Println("No blogs tracked yet. Use 'blogwatcher add' to add one.")
					return nil
				}
				if !silent {
					color.New(color.FgCyan).Printf("Scanning %d blog(s)...\n\n", len(blogs))
				}
				var results []scanner.ScanResult
				if category != "" {
					results, err = scanner.ScanBlogsByCategory(db, category, workers)
				} else if uncategorized {
					results, err = scanner.ScanUncategorizedBlogs(db, workers)
				} else {
					results, err = scanner.ScanAllBlogs(db, workers)
				}
				if err != nil {
					return err
				}
				totalNew := 0
				for _, result := range results {
					if !silent {
						printScanResult(result)
					}
					totalNew += result.NewArticles
				}
				if !silent {
					fmt.Println()
					if totalNew > 0 {
						color.New(color.FgGreen, color.Bold).Printf("Found %d new article(s) total!\n", totalNew)
					} else {
						color.New(color.FgYellow).Println("No new articles found.")
					}
				}
			}

			if silent {
				fmt.Println("scan done")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&silent, "silent", "s", false, "Only output 'scan done' when complete")
	cmd.Flags().IntVarP(&workers, "workers", "w", 8, "Number of concurrent workers when scanning all blogs")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Only scan blogs in this category")
	cmd.Flags().BoolVar(&uncategorized, "uncategorized", false, "Only scan blogs with no categories")
	return cmd
}

func newArticlesCommand() *cobra.Command {
	var showAll bool
	var blogName string
	var category string
	var uncategorized bool

	cmd := &cobra.Command{
		Use:   "articles",
		Short: "List articles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if category != "" && uncategorized {
				return fmt.Errorf("--category and --uncategorized are mutually exclusive")
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			articles, blogNames, err := controller.GetArticles(db, showAll, blogName, category, uncategorized)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if len(articles) == 0 {
				if showAll {
					fmt.Println("No articles found.")
				} else {
					color.New(color.FgGreen).Println("No unread articles!")
				}
				return nil
			}

			label := "Unread articles"
			if showAll {
				label = "All articles"
			}
			color.New(color.FgCyan, color.Bold).Printf("%s (%d):\n\n", label, len(articles))
			for _, article := range articles {
				printArticle(article, blogNames[article.BlogID])
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all articles (including read)")
	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Filter by blog name")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by category")
	cmd.Flags().BoolVar(&uncategorized, "uncategorized", false, "Show only articles from uncategorized blogs")
	return cmd
}

func newReadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <article_id>",
		Short: "Mark an article as read.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleRead(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if article.IsRead {
				fmt.Printf("Article %d is already marked as read.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as read\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newReadAllCommand() *cobra.Command {
	var blogName string
	var category string
	var uncategorized bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "read-all",
		Short: "Mark all unread articles as read.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if category != "" && uncategorized {
				return fmt.Errorf("--category and --uncategorized are mutually exclusive")
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			articles, blogNames, err := controller.GetArticles(db, false, blogName, category, uncategorized)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if len(articles) == 0 {
				color.New(color.FgGreen).Println("No unread articles to mark as read.")
				return nil
			}

			if !yes {
				scope := "all blogs"
				if blogName != "" {
					scope = fmt.Sprintf("from '%s'", blogName)
				} else if category != "" {
					scope = fmt.Sprintf("in category '%s'", category)
				} else if uncategorized {
					scope = "from uncategorized blogs"
				}
				confirmed, err := confirm(fmt.Sprintf("Mark %d article(s) %s as read?", len(articles), scope))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			marked, err := controller.MarkAllArticlesRead(db, blogName, category, uncategorized)
			if err != nil {
				printError(err)
				return markError(err)
			}

			_ = blogNames
			color.New(color.FgGreen).Printf("Marked %d article(s) as read\n", len(marked))
			return nil
		},
	}

	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Only mark articles from this blog")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Only mark articles in this category")
	cmd.Flags().BoolVar(&uncategorized, "uncategorized", false, "Only mark articles from uncategorized blogs")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newUnreadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unread <article_id>",
		Short: "Mark an article as unread.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleUnread(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if !article.IsRead {
				fmt.Printf("Article %d is already marked as unread.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as unread\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newCategoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "category",
		Short: "Manage blog categories.",
	}
	cmd.AddCommand(newCategoryAddCommand())
	cmd.AddCommand(newCategoryRemoveCommand())
	cmd.AddCommand(newCategoryListCommand())
	cmd.AddCommand(newCategoryAssignCommand())
	cmd.AddCommand(newCategoryUnassignCommand())
	return cmd
}

func newCategoryAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Create a new category.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = controller.AddCategory(db, args[0])
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Created category '%s'\n", args[0])
			return nil
		},
	}
}

func newCategoryRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a category (blogs are kept).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove category '%s'? Blogs will not be deleted.", name))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.RemoveCategory(db, name); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed category '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newCategoryListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all categories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			categories, err := controller.ListCategories(db)
			if err != nil {
				return err
			}
			if len(categories) == 0 {
				fmt.Println("No categories yet. Use 'blogwatcher category add' to create one.")
				return nil
			}
			color.New(color.FgCyan, color.Bold).Printf("Categories (%d):\n\n", len(categories))
			for _, cat := range categories {
				blogs, _ := db.ListBlogsByCategory(cat.ID)
				color.New(color.FgWhite, color.Bold).Printf("  %s", cat.Name)
				color.New(color.FgHiBlack).Printf(" (%d blogs)\n", len(blogs))
			}
			fmt.Println()
			return nil
		},
	}
}

func newCategoryAssignCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "assign <blog_name> <category_name>",
		Short: "Assign a blog to a category.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.AssignBlogToCategory(db, args[0], args[1]); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Assigned '%s' to category '%s'\n", args[0], args[1])
			return nil
		},
	}
}

func newCategoryUnassignCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unassign <blog_name> <category_name>",
		Short: "Remove a blog from a category.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.UnassignBlogFromCategory(db, args[0], args[1]); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Unassigned '%s' from category '%s'\n", args[0], args[1])
			return nil
		},
	}
}

func printScanResult(result scanner.ScanResult) {
	statusColor := color.FgWhite
	if result.NewArticles > 0 {
		statusColor = color.FgGreen
	}
	color.New(color.FgWhite, color.Bold).Printf("  %s\n", result.BlogName)
	if result.Error != "" {
		color.New(color.FgRed).Printf("    Error: %s\n", result.Error)
		return
	}
	if result.Source == "none" {
		color.New(color.FgYellow).Println("    No feed or scraper configured")
		return
	}
	sourceLabel := "HTML"
	if result.Source == "rss" {
		sourceLabel = "RSS"
	}
	fmt.Printf("    Source: %s | Found: %d | ", sourceLabel, result.TotalFound)
	color.New(statusColor).Printf("New: %d\n", result.NewArticles)
}

func printArticle(article model.Article, blogName string) {
	status := color.New(color.FgYellow).Sprint("[new]")
	if article.IsRead {
		status = color.New(color.FgHiBlack).Sprint("[read]")
	}
	idStr := color.New(color.FgCyan).Sprintf("[%d]", article.ID)
	fmt.Printf("  %s %s %s\n", idStr, status, article.Title)
	fmt.Printf("       Blog: %s\n", blogName)
	fmt.Printf("       URL: %s\n", article.URL)
	if article.PublishedDate != nil {
		fmt.Printf("       Published: %s\n", article.PublishedDate.Format("2006-01-02"))
	}
	fmt.Println()
}

func printError(err error) {
	color.New(color.FgRed).Printf("Error: %s\n", err.Error())
}

func parseID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid article id: %s", value)
	}
	return parsed, nil
}

func confirm(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

func init() {
	cobra.EnableCommandSorting = false
	cobra.AddTemplateFunc("now", func() string { return time.Now().Format(time.RFC3339) })
}
