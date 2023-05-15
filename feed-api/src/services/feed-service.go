package services

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Feed struct {
	Id        string `json:"id"`
	Url       string `db:"url" json:"url"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

func GetUserFeeds(db *sqlx.DB, userId string) ([]Feed, error) {
	feeds := []Feed{}
	err := db.Select(
		&feeds, `
		SELECT
			feeds.*
		FROM feeds
		INNER JOIN user_feeds uf ON (uf.feed_id = feeds.id)
		WHERE uf.user_id = $1`,
		userId,
	)

	if err != nil {
		return feeds, err
	}

	return feeds, nil
}

func AddUserFeed(db *sqlx.DB, userId string, feedUrl string) (Feed, error) {
	feed := Feed{}
	err := db.Get(
		&feed, `
		WITH feed AS (
			INSERT INTO feeds (url) VALUES ($1) RETURNING *
		)
		INSERT INTO user_feeds (user_id, feed_id) VALUES ($2, feed.id)
		RETURNING feed.*`,
		feedUrl,
		userId,
	)

	if err != nil {
		return feed, err
	}

	return feed, nil
}
