package services

import (
	"database/sql/driver"
	"encoding/json"
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

// Tag service types and functions
type Tag struct {
	Id        string `json:"id"`
	UserId    string `db:"user_id" json:"userId"`
	Name      string `json:"name"`
	CreatedAt string `db:"created_at" json:"createdAt"`
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

// Extended feed type with tags
type FeedWithTags struct {
	Feed
	Tags TagsArray `json:"tags" db:"tags"`
}

// TagsArray is a custom type that can scan JSON arrays into []Tag
type TagsArray []Tag

// Scan implements the sql.Scanner interface for TagsArray
func (ta *TagsArray) Scan(src interface{}) error {
	if src == nil {
		*ta = []Tag{}
		return nil
	}

	switch v := src.(type) {
	case []byte:
		// Handle JSON data
		var tags []Tag
		err := json.Unmarshal(v, &tags)
		if err != nil {
			return err
		}
		*ta = tags
	case string:
		// Handle JSON string
		var tags []Tag
		err := json.Unmarshal([]byte(v), &tags)
		if err != nil {
			return err
		}
		*ta = tags
	default:
		return fmt.Errorf("unsupported type for TagsArray: %T", src)
	}

	return nil
}

// Value implements the driver.Valuer interface for TagsArray
func (ta TagsArray) Value() (driver.Value, error) {
	if len(ta) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(ta)
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

func GetContent(db *sqlx.DB, userId string, page int, pageSize int, tagId string) ([]FeedContentWithSource, error) {
	feedContent := []FeedContentWithSource{}
	err := db.Select(
		&feedContent,
		`SELECT fc.id, fc.feed_id, fc.guid, fc.title, fc.img_url, fc.link, fc.created_at,
			 COALESCE(fc.published_at, fc.created_at) as published_at,
			 f.title as feed_title
			 FROM feed_content fc
			 INNER JOIN feeds f ON (f.id = fc.feed_id)
			 INNER JOIN user_feeds uf ON (uf.feed_id = fc.feed_id)
		 	 LEFT JOIN feed_tags ft ON (ft.feed_id = uf.feed_id)
		  	 LEFT JOIN tags t ON (ft.tag_id = t.id and t.user_id = $1)
		 	 WHERE (
		 	 	CASE WHEN $4 = '*' THEN TRUE
		 	 	ELSE t.id = $4::uuid
		 	 	END
		 	 ) AND
		 	 uf.user_id = $1
			 ORDER BY COALESCE(fc.published_at, fc.created_at) DESC
			 LIMIT $2 OFFSET $3`,
		userId,
		pageSize,
		(page-1)*pageSize,
		tagId,
	)

	if err != nil {
		return feedContent, err
	}

	return feedContent, nil
}

func GetContentCount(db *sqlx.DB, userId string, tagId string) (int, error) {
	var count int
	err := db.Get(
		&count,
		`SELECT COUNT(*)
		 	FROM feed_content fc
		 INNER JOIN user_feeds uf ON (uf.feed_id = fc.feed_id)
		 LEFT JOIN feed_tags ft ON (ft.feed_id = uf.feed_id)
		 LEFT JOIN tags t ON (ft.tag_id = t.id and t.user_id = $1)
		 WHERE (
		 	CASE WHEN $2 = '*' THEN TRUE
		 	ELSE t.id = $2::uuid
		 	END
		 ) AND
		 uf.user_id = $1`,
		userId,
		tagId,
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

// Tag service functions

func GetUserTags(db *sqlx.DB, userId string) ([]Tag, error) {
	tags := []Tag{}
	err := db.Select(
		&tags,
		`SELECT * FROM tags WHERE user_id = $1 ORDER BY name ASC`,
		userId,
	)

	if err != nil {
		return tags, err
	}

	return tags, nil
}

func CreateTag(db *sqlx.DB, userId string, name string) (Tag, error) {
	tag := Tag{}
	err := db.Get(
		&tag,
		`INSERT INTO tags (user_id, name) VALUES ($1, $2)
		 RETURNING *`,
		userId,
		name,
	)

	if err != nil {
		return tag, err
	}

	return tag, nil
}

func DeleteTag(db *sqlx.DB, userId string, tagId string) error {
	_, err := db.Exec(
		`DELETE FROM tags WHERE id = $1 AND user_id = $2`,
		tagId,
		userId,
	)

	if err != nil {
		return err
	}

	return nil
}

func GetFeedTags(db *sqlx.DB, feedId string) ([]Tag, error) {
	tags := []Tag{}
	err := db.Select(
		&tags,
		`SELECT t.* FROM tags t
		 INNER JOIN feed_tags ft ON (ft.tag_id = t.id)
		 WHERE ft.feed_id = $1`,
		feedId,
	)

	if err != nil {
		return tags, err
	}

	return tags, nil
}

func AddTagToFeed(db *sqlx.DB, feedId string, tagId string) error {
	_, err := db.Exec(
		`INSERT INTO feed_tags (feed_id, tag_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		feedId,
		tagId,
	)

	if err != nil {
		return err
	}

	return nil
}

func RemoveTagFromFeed(db *sqlx.DB, feedId string, tagId string) error {
	_, err := db.Exec(
		`DELETE FROM feed_tags WHERE feed_id = $1 AND tag_id = $2`,
		feedId,
		tagId,
	)

	if err != nil {
		return err
	}

	return nil
}

func GetUserFeedsWithTags(db *sqlx.DB, userId string) ([]FeedWithTags, error) {
	feeds := []FeedWithTags{}
	err := db.Select(
		&feeds,
		`SELECT 
			f.*,
			COALESCE(
				(SELECT json_agg(t) FROM (
					SELECT t.id, t.user_id, t.name, t.created_at 
					FROM tags t 
					INNER JOIN feed_tags ft ON ft.tag_id = t.id 
					WHERE ft.feed_id = f.id AND t.user_id = $1
				) t),
				'[]'::json
			) as tags
		FROM feeds f
		INNER JOIN user_feeds uf ON (uf.feed_id = f.id)
		WHERE uf.user_id = $1
		ORDER BY f.title ASC`,
		userId,
	)

	if err != nil {
		return feeds, err
	}

	return feeds, nil
}
