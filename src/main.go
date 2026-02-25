package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"rss-simple/src/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/template/html/v2"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")

	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	// Setup template engine
	engine := html.New("./src/templates", ".html")
	
	// Add custom template functions
	engine.AddFunc("minus", func(a, b int) int { return a - b })
	engine.AddFunc("add", func(a, b int) int { return a + b })
	engine.AddFunc("mul", func(a, b int) int { return a * b })
	engine.AddFunc("seq", func(start, end int) []int {
		result := make([]int, end-start+1)
		for i := 0; i <= end-start; i++ {
			result[i] = start + i
		}
		return result
	})
	engine.AddFunc("formatDate", func(dateStr string) string {
		if dateStr == "" {
			return ""
		}
		// Try to parse various date formats
		formats := []string{
			"Mon, 02 Jan 2006 15:04:05 -0700", // RFC1123
			"Mon, 02 Jan 2006 15:04:05 MST",   // RFC1123 without timezone offset
			"2006-01-02T15:04:05-07:00",       // ISO8601
			"2006-01-02T15:04:05Z07:00",       // ISO8601 with Z
			"2006-01-02 15:04:05",             // Simple datetime
			"02 Jan 2006 15:04:05 MST",        // Common RSS format
		}
		
		var parsedTime time.Time
		var err error
		for _, format := range formats {
			parsedTime, err = time.Parse(format, dateStr)
			if err == nil {
				break
			}
		}
		
		if err != nil {
			// If we can't parse it, return the original string
			return dateStr
		}
		
		// Format in a user-friendly way
		return parsedTime.Format("Jan 2, 2006 3:04 PM")
	})

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Setup session/store for cookies
	store := session.New()

	app.Static("/static", "./static")

	// Middleware to check authentication
	authMiddleware := func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.Redirect("/login")
		}

		userID := sess.Get("user_id")
		if userID == nil {
			return c.Redirect("/login")
		}

		c.Locals("user_id", userID)
		return c.Next()
	}

	// Public routes
	app.Get("/", func(c *fiber.Ctx) error {
		// Check if user is authenticated by trying to get session
		sess, err := store.Get(c)
		if err != nil {
			return c.Redirect("/login")
		}

		userID := sess.Get("user_id")
		if userID == nil {
			return c.Redirect("/login")
		}

		return c.Redirect("/content")
	})

	app.Get("/login", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.Render("login", fiber.Map{
				"Title": "Login",
			}, "base")
		}

		data := fiber.Map{
			"Title": "Login",
		}

		// Check if there's a new user ID to display (from registration redirect)
		if newUserID := sess.Get("new_user_id"); newUserID != nil {
			data["NewUserID"] = newUserID
			sess.Delete("new_user_id") // Clear it after showing once
			if err := sess.Save(); err != nil {
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		return c.Render("login", data, "base")
	})

	app.Post("/login", func(c *fiber.Ctx) error {
		userId := c.FormValue("userId")
		if userId == "" {
			return c.Render("login", fiber.Map{
				"Title":   "Login",
				"Error":   "User ID is required",
			}, "base")
		}

		usr, err := services.GetUser(db, userId)
		if err != nil {
			return c.Render("login", fiber.Map{
				"Title":   "Login",
				"Error":   "User not found",
			}, "base")
		}

		// Create session with long expiry (30 days)
		sess, err := store.Get(c)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
	}

		sess.Set("user_id", usr.Id)
		sess.SetExpiry(time.Hour * 24 * 30) // 30 days
		if err := sess.Save(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		services.UpdateActive(db, usr.Id)
		return c.Redirect("/content")
	})

	app.Get("/register", func(c *fiber.Ctx) error {
		return c.Render("register", fiber.Map{
			"Title": "Register",
		}, "base")
	})

	app.Post("/register", func(c *fiber.Ctx) error {
		usr, err := services.CreateUser(db)
		if err != nil {
			return c.Render("register", fiber.Map{
				"Title":   "Register",
				"Error":   "Failed to create user",
			}, "base")
		}

		// Store the new user ID in session temporarily so we can show it on login page
		sess, err := store.Get(c)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		sess.Set("new_user_id", usr.Id)
		if err := sess.Save(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Redirect("/login")
	})

	// Protected routes
	app.Get("/content", authMiddleware, func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)

		// Get pagination parameters from query, default to page 1 with 10 items per page
		page, err := strconv.Atoi(c.Query("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}

		pageSize, err := strconv.Atoi(c.Query("page_size", "25"))
		if err != nil || pageSize < 1 {
			pageSize = 25
		}

		content, err := services.GetContent(db, userID, page, pageSize)
		if err != nil {
			fmt.Println(err)
						
			return c.Render("index", fiber.Map{
				"Title":       "Your RSS Feed",
				"Error":       "Failed to load content",
				"Content":     []services.FeedContentWithSource{},
				"CurrentPage": 0,
				"TotalPages":  0,
				"PageSize":    0,
				"TotalCount":  0,
			}, "base")
		}

		// Get total count for pagination
		totalCount, err := services.GetContentCount(db, userID)
		if err != nil {
			fmt.Println(err)

			return c.Render("index", fiber.Map{
				"Title":       "Your RSS Feed",
				"Error":       "Failed to load content",
				"Content":     []services.FeedContentWithSource{},
				"CurrentPage": 0,
				"TotalPages":  0,
				"PageSize":    0,
				"TotalCount":  0,
			}, "base")
		}

		// Calculate total pages
		totalPages := 1
		if pageSize > 0 && totalCount > 0 {
			totalPages = (totalCount + pageSize - 1) / pageSize
		}

		return c.Render("index", fiber.Map{
			"Title":       "Your RSS Feed",
			"Content":     content,
			"CurrentPage": page,
			"TotalPages":  totalPages,
			"PageSize":    pageSize,
			"TotalCount":  totalCount,
		}, "base")
	})

	app.Get("/feeds", authMiddleware, func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)

		feeds, err := services.GetUserFeeds(db, userID)
		if err != nil {
			fmt.Println(err)
			return c.Render("feeds", fiber.Map{
				"Title": "Your Feeds",
				"Error": "Failed to load feeds",
				"Feeds": []services.Feed{},
			}, "base")
		}

		return c.Render("feeds", fiber.Map{
			"Title": "Your Feeds",
			"Feeds": feeds,
		}, "base")
	})

	app.Get("/add-feed", authMiddleware, func(c *fiber.Ctx) error {
		return c.Render("add_feed", fiber.Map{
			"Title": "Add RSS Feed",
		}, "base")
	})

	app.Post("/add-feed", authMiddleware, func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		url := c.FormValue("url")

		if url == "" {
			return c.Render("add_feed", fiber.Map{
				"Title": "Add RSS Feed",
				"Error": "URL is required",
			}, "base")
		}

		_, err := services.AddUserFeed(db, userID, url)
		if err != nil {
			return c.Render("add_feed", fiber.Map{
				"Title": "Add RSS Feed",
				"Error": "Failed to add feed: " + err.Error(),
			}, "base")
		}

		return c.Render("add_feed", fiber.Map{
			"Title":   "Add RSS Feed",
			"Success": "Feed added successfully!",
		}, "base")
	})

	app.Post("/feeds/:feedId/delete", authMiddleware, func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		feedId := c.Params("feedId")

		err := services.DeleteUserFeed(db, userID, feedId)
		if err != nil {
			fmt.Println(err)
		}

		return c.Redirect("/feeds")
	})

	app.Get("/update", authMiddleware, func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)

		err := services.UpdateUserContent(db, userID)
		if err != nil {
			fmt.Println(err)
			return c.Render("index", fiber.Map{
				"Title":   "Update Content",
				"Error":   "Failed to update content",
				"Content": []services.FeedContentWithSource{},
			}, "base")
		}

		return c.Redirect("/content")
	})

	app.Get("/logout", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.Redirect("/login")
		}

		if err := sess.Destroy(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Redirect("/login")
	})

	log.Fatalln(app.Listen(":" + port))
}
