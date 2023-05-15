package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"usr-api/src/services"

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

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBrokerUrl},
		GroupID:  "rss-serve",
		Topic:    "update-user-activity",
		MinBytes: 1,
		MaxBytes: 5e6,
	})

	app := fiber.New()

	guarded := jwtware.New(jwtware.Config{
		SigningKey: []byte(jwtSecret),
	})

	app.Get("/api/users/me", guarded, func(c *fiber.Ctx) error {
		jwtContent := c.Locals("user").(*jwt.Token)
		claims := jwtContent.Claims.(jwt.MapClaims)
		id := claims["uid"].(string)

		usr, err := services.GetUser(db, id)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusNotFound)
		}

		return c.JSON(usr)
	})

	app.Post("/api/users/", func(c *fiber.Ctx) error {
		usr, err := services.CreateUser(db)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.JSON(usr)
	})

	app.Post("/api/users/login", func(c *fiber.Ctx) error {
		var uid string
		err := c.BodyParser(&uid)
		if err != nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		usr, err := services.GetUser(db, uid)
		if err != nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		claims := jwt.MapClaims{
			"uid": usr.Id,
			"exp": time.Now().Add(time.Hour * 3).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signedToken, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		services.UpdateActive(db, usr.Id)

		return c.JSON(fiber.Map{
			"token": signedToken,
			"user":  usr,
		})
	})

	go startConsuming(kafkaReader, db)
	log.Fatalln(app.Listen(":3000"))
}

func startConsuming(reader *kafka.Reader, db *sqlx.DB) {
	defer reader.Close()

	for {
		message, err := reader.ReadMessage(context.Background())

		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(message)
	}
}
