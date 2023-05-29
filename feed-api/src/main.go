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
	kafka "github.com/segmentio/kafka-go"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	kafkaBrokerUrl := os.Getenv("KAFKA_BROKER_URL")

	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	kafkaWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{kafkaBrokerUrl},
		Topic:    "user-feeds",
		Balancer: &kafka.LeastBytes{},
	})

	app := fiber.New()

	guarded := jwtware.New(jwtware.Config{
		SigningKey: []byte(jwtSecret),
	})

	app.Get("/api/users/me/feeds", guarded, func(c *fiber.Ctx) error {
		id := getUid(c)

		feeds, err := services.GetUserFeeds(db, id)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusNotFound)
		}

		return c.JSON(feeds)
	})

	app.Post("/api/users/me/feeds", guarded, func(c *fiber.Ctx) error {
		id := getUid(c)

		body := services.NewFeedBody{}
		c.BodyParser(&body)

		newFeed, err := services.AddUserFeed(db, kafkaWriter, id, body.Url)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusBadRequest)
		}

		return c.JSON(newFeed)

	})

	app.Delete("/api/users/me/feeds/:feedId", guarded, func(c *fiber.Ctx) error {
		id := getUid(c)
		feedId := c.Params("feedId")

		err := services.DeleteUserFeed(db, kafkaWriter, id, feedId)

		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/api/users/me/content/update", guarded, func(c *fiber.Ctx) error {
		id := getUid(c)

		err := services.SendUpdateEvent(db, kafkaWriter, id)

		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.SendStatus(fiber.StatusOK)
	})

	defer kafkaWriter.Close()
	log.Fatalln(app.Listen(":4000"))
}

func getUid(c *fiber.Ctx) string {
	jwtContent := c.Locals("user").(*jwt.Token)
	claims := jwtContent.Claims.(jwt.MapClaims)
	id := claims["uid"].(string)

	return id
}
