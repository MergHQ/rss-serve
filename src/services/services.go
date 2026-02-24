package services

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
)

// User service types and functions
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

// Feed service types and functions
type Feed struct {
	Id        string `json:"id"`
	Url       string `json:"url"`
	Title     string `json:"title"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

type NewFeedBody struct {
	Url string `json:"url"`
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

func AddUserFeed(db *sqlx.DB, userId string, feedUrl string) (Feed, error) {
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

	return feed, nil
}

func DeleteUserFeed(db *sqlx.DB, userId string, feedId string) error {
	_, err := db.Exec(
		"DELETE FROM user_feeds WHERE user_id = $1 AND feed_id = $2",
		userId,
		feedId,
	)

	if err != nil {
		return err
	}

	return nil
}

// Content service types and functions
type FeedContent struct {
	Id          string `json:"id"`
	FeedId      string `json:"feedId" db:"feed_id"`
	Guid        string `json:"guid"`
	Title       string `json:"title"`
	ImgUrl      string `db:"img_url" json:"imgUrl"`
	Link        string `json:"link"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
	PublishedAt string `db:"published_at" json:"publishedAt"`
}

type FeedContentWithSource struct {
	FeedContent
	FeedTitle string `db:"feed_title" json:"feedTitle"`
}

type NewFeedContent struct {
	FeedId      string `db:"feed_id"`
	Guid        string
	Title       string
	ImgUrl      string `db:"img_url"`
	Link        string
	PublishedAt string `db:"published_at"`
}

type UpdateableContent struct {
	FeedId string `json:"feedId"`
	Url    string `json:"url"`
}

func GetContent(db *sqlx.DB, userId string, page int, pageSize int) ([]FeedContentWithSource, error) {
	feedContent := []FeedContentWithSource{}
	err := db.Select(
		&feedContent,
		`SELECT fc.id, fc.feed_id, fc.guid, fc.title, fc.img_url, fc.link, fc.created_at,
			 COALESCE(fc.published_at, fc.created_at) as published_at,
			 f.title as feed_title
			 FROM feed_content fc
			 INNER JOIN feeds f ON (f.id = fc.feed_id)
			 INNER JOIN user_feeds uf ON (uf.feed_id = fc.feed_id)
			 WHERE uf.user_id = $1
			 ORDER BY COALESCE(fc.published_at, fc.created_at) DESC
			 LIMIT $2 OFFSET $3`,
		userId,
		pageSize,
		(page-1)*pageSize,
	)

	if err != nil {
		return feedContent, err
	}

	return feedContent, nil
}

func GetContentCount(db *sqlx.DB, userId string) (int, error) {
	var count int
	err := db.Get(
		&count,
		`SELECT COUNT(*)
		 FROM feed_content fc
		 INNER JOIN user_feeds uf ON (uf.feed_id = fc.feed_id)
		 WHERE uf.user_id = $1`,
		userId,
	)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func getFeedContent(feedUrl string, feedId string) ([]NewFeedContent, error) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(feedUrl)

	if err != nil {
		return []NewFeedContent{}, err
	}

	newItems := []NewFeedContent{}
	for _, item := range feed.Items {
		imgUrl := ""
		if item.Image != nil {
			imgUrl = item.Image.URL
		} else if len(item.Enclosures) > 0 {
			imgUrl = item.Enclosures[0].URL
		}

		// Extract publication date, preferring Published over Updated
		publishedAt := ""
		if item.Published != "" {
			publishedAt = item.Published
		} else if item.Updated != "" {
			publishedAt = item.Updated
		}

		newItems = append(
			newItems,
			NewFeedContent{
				FeedId:      feedId,
				Guid:        item.GUID,
				Title:       item.Title,
				ImgUrl:      imgUrl,
				Link:        item.Link,
				PublishedAt: publishedAt,
			},
		)
	}

	return newItems, nil
}

func UpdateUserContent(db *sqlx.DB, userId string) error {
	userFeeds, err := GetUserFeeds(db, userId)

	if err != nil {
		return err
	}

	tx, err := db.Beginx()

	if err != nil {
		return err
	}

	for _, feed := range userFeeds {
		newItemsToInsert, _ := getFeedContent(feed.Url, feed.Id)
		fmt.Printf("Updating content for feed %s, found %d items\n", feed.Id, len(newItemsToInsert))
		tx.NamedExec(
			`INSERT INTO feed_content (feed_id, "guid", title, img_url, "link", published_at)
			 VALUES (:feed_id, :guid, :title, :img_url, :link, :published_at) ON CONFLICT DO NOTHING`,
			newItemsToInsert,
		)
	}

	err = tx.Commit()

	if err != nil {
		return err
	}

	return nil
}
