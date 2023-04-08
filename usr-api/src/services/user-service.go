package services

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type User struct {
	Id           string `json:"id"`
	LastActiveAt string `db:"last_active_at" json:"lastActiveAt"`
	CreatedAt    string `db:"created_at" json:"createdAt"`
}

func GetUser(db *sqlx.DB, id string) (User, error) {
	user := User{}
	err := db.Get(&user, "SELECT * FROM users WHERE id = $1", id)
	if err != nil {
		return user, err
	}

	return user, nil
}

func CreateUser(db *sqlx.DB) (User, error) {
	user := User{}
	err := db.Get(&user, "INSERT INTO users DEFAULT VALUES RETURNING *")
	if err != nil {
		return user, err
	}

	return user, nil
}

func UpdateActive(db *sqlx.DB, id string) error {
	_, err := db.Exec("UPDATE users SET last_active_at = NOW() where id = $1", id)
	return err
}
