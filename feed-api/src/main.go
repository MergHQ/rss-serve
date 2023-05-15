package main

import (
	"fmt"
	"log"
	"os"

	"feed-api/src/services"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"

	jwtware "github.com/gofiber/jwt/v3"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	kafkaBrokerUrl := os.Getenv("KAFKA_BROKER_URL")
	fmt.Println(kafkaBrokerUrl)
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	/* kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBrokerUrl},
		GroupID:  "rss-serve",
		Topic:    "update-user-activity",
		MinBytes: 1,
		MaxBytes: 5e6,
	}) */

	app := fiber.New()

	guarded := jwtware.New(jwtware.Config{
		SigningKey: []byte(jwtSecret),
	})

	app.Get("/api/users/me/feeds", guarded, func(c *fiber.Ctx) error {
		jwtContent := c.Locals("user").(*jwt.Token)
		claims := jwtContent.Claims.(jwt.MapClaims)
		id := claims["uid"].(string)

		feeds, err := services.GetUserFeeds(db, id)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusNotFound)
		}

		return c.JSON(feeds)
	})

	app.Post("/api/users/me/feeds", guarded, func(c *fiber.Ctx) error {
		jwtContent := c.Locals("user").(*jwt.Token)
		claims := jwtContent.Claims.(jwt.MapClaims)
		id := claims["uid"].(string)

	})

	log.Fatalln(app.Listen(":4000"))
}
