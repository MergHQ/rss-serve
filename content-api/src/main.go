package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"content-api/src/services"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	jsoniter "github.com/json-iterator/go"

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

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBrokerUrl},
		GroupID:  "rss-serve",
		Topic:    "user-feeds",
		MinBytes: 1,
		MaxBytes: 5e6,
	})

	app := fiber.New()

	guarded := jwtware.New(jwtware.Config{
		SigningKey: []byte(jwtSecret),
	})

	app.Get("/api/users/me/content", guarded, func(c *fiber.Ctx) error {
		jwtContent := c.Locals("user").(*jwt.Token)
		claims := jwtContent.Claims.(jwt.MapClaims)
		id := claims["uid"].(string)

		content, err := services.GetContent(db, id)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusNotFound)
		}

		return c.JSON(content)
	})

	go startConsuming(kafkaReader, db)
	defer kafkaReader.Close()

	log.Fatalln(app.Listen(":5000"))
}

func startConsuming(reader *kafka.Reader, db *sqlx.DB) {

	for {
		message, err := reader.ReadMessage(context.Background())

		if err != nil {
			log.Fatalln(err)
		}

		key := string(message.Key[:])

		fmt.Println("Kafka event:", string(message.Value[:]))

		if strings.HasPrefix(key, "add-feed") || strings.HasPrefix(key, "delete-feed") {
			payload := services.UserFeedLinkPayload{}
			err = jsoniter.Unmarshal(message.Value, &payload)

			if err != nil {
				fmt.Println(err)
			}

			if strings.HasPrefix(key, "add-feed") {
				err = services.CreateUserFeedLink(db, payload)
			} else {
				err = services.DeleteUserFeedLink(db, payload)
			}

			if err != nil {
				fmt.Println(err)
			}

		}

		if strings.HasPrefix(key, "update-feed") {
			payload := services.UpdateUserContentPayload{}
			err = jsoniter.Unmarshal(message.Value, &payload)

			if err != nil {
				fmt.Println(err)
			}

			err = services.UpdateUserContent(db, payload)

			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
