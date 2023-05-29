package services

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	jsoniter "github.com/json-iterator/go"
	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
	kafka "github.com/segmentio/kafka-go"
)

type Feed struct {
	Id        string `json:"id"`
	Url       string `json:"url"`
	Title     string `json:"title"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

type NewFeedBody struct {
	Url string `json:"url"`
}

type UpdateableContent struct {
	FeedId string `json:"feedId"`
	Url    string `json:"url"`
}

type UpdateUserContentPayload struct {
	Op              string              `json:"op"`
	ContentToUpdate []UpdateableContent `json:"contenToUpdate"`
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

func getRssFeedTitle(feedUrl string) (string, error) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(feedUrl)

	if err != nil {
		return "", err
	}

	return feed.Title, nil
}

func AddUserFeed(db *sqlx.DB, writerInstance *kafka.Writer, userId string, feedUrl string) (Feed, error) {
	feed := Feed{}

	feedTitle, err := getRssFeedTitle(feedUrl)

	if err != nil {
		return feed, err
	}

	err = db.Get(
		&feed, `
		WITH feed_insert AS (
			INSERT INTO feeds (url, title) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
			RETURNING *
		), new_feed AS (
			SELECT * FROM feeds WHERE url = $1
		), usr_feed AS (
			INSERT INTO user_feeds (user_id, feed_id)
			VALUES ($3, COALESCE((SELECT id FROM feed_insert), (SELECT id FROM new_feed)))
		)
		SELECT * FROM feed_insert UNION SELECT * FROM new_feed nf`,
		feedUrl,
		feedTitle,
		userId,
	)

	if err != nil {
		return feed, err
	}

	message := kafka.Message{
		Key:   []byte("add-feed" + feed.Id + "-" + userId),
		Value: []byte(fmt.Sprintf(`{"op": "create", "feedId": "%s", "userId": "%s"}`, feed.Id, userId)),
	}
	err = writerInstance.WriteMessages(context.Background(), message)

	if err != nil {
		// Rollback new feed if propagation fails
		db.Exec("DELETE FROM user_feeds WHERE feed_id = $1 AND user_id = $2", feed.Id, userId)

		return feed, err
	}

	return feed, nil
}

func DeleteUserFeed(db *sqlx.DB, writerInstance *kafka.Writer, userId string, feedId string) error {
	_, err := db.Exec(
		"DELETE FROM user_feeds WHERE user_id = $1 AND feed_id = $2",
		userId,
		feedId,
	)

	if err != nil {
		return err
	}

	message := kafka.Message{
		Key:   []byte("delete-feed" + feedId + "-" + userId),
		Value: []byte(fmt.Sprintf(`{"op": "delete", "feedId": "%s", "userId": "%s"}`, feedId, userId)),
	}

	err = writerInstance.WriteMessages(context.Background(), message)

	if err != nil {
		db.Exec("INSERT INTO user_feeds (user_id, feed_id) VALUES ($1, $2)", userId, feedId)
		return err
	}

	return nil
}

func SendUpdateEvent(db *sqlx.DB, writerInstance *kafka.Writer, userId string) error {
	userFeeds, err := GetUserFeeds(db, userId)

	if err != nil {
		return err
	}

	feedsToUpdate := []UpdateableContent{}

	for _, feed := range userFeeds {
		feedsToUpdate = append(
			feedsToUpdate,
			UpdateableContent{
				FeedId: feed.Id,
				Url:    feed.Url,
			},
		)
	}

	payload := UpdateUserContentPayload{
		Op:              "update",
		ContentToUpdate: feedsToUpdate,
	}

	json, err := jsoniter.Marshal(payload)

	if err != nil {
		return err
	}

	message := kafka.Message{
		Key:   []byte("update-feed" + "-" + userId),
		Value: json,
	}

	err = writerInstance.WriteMessages(context.Background(), message)

	if err != nil {
		return err
	}

	return nil
}
