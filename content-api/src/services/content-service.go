package services

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
)

type FeedContent struct {
	Id        string `json:"id"`
	FeedId    string `json:"feedId" db:"feed_id"`
	Guid      string `json:"guid"`
	Title     string `json:"title"`
	ImgUrl    string `db:"img_url" json:"imgUrl"`
	Link      string `json:"link"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

type NewFeedContent struct {
	FeedId string `db:"feed_id"`
	Guid   string
	Title  string
	ImgUrl string `db:"img_url"`
	Link   string
}

type UserFeedLinkPayload struct {
	FeedId string `json:"feedId"`
	UserId string `json:"userId"`
}

type UpdateableContent struct {
	FeedId string `json:"feedId"`
	Url    string `json:"url"`
}

type UpdateUserContentPayload struct {
	Op              string              `json:"op"`
	ContentToUpdate []UpdateableContent `json:"contenToUpdate"`
}

func GetContent(db *sqlx.DB, userId string) ([]FeedContent, error) {
	feedContent := []FeedContent{}
	err := db.Select(
		&feedContent,
		`SELECT fc.*
		 FROM feed_content fc
		 INNER JOIN user_feeds uf ON (uf.feed_id = fc.feed_id)
		 WHERE uf.user_id = $1`,
		userId,
	)

	if err != nil {
		return feedContent, err
	}

	return feedContent, nil
}

func CreateUserFeedLink(db *sqlx.DB, createUserFeedLinkPayload UserFeedLinkPayload) error {
	_, err := db.Exec(
		"INSERT INTO user_feeds (feed_id, user_id) VALUES ($1, $2)",
		createUserFeedLinkPayload.FeedId,
		createUserFeedLinkPayload.UserId,
	)

	if err != nil {
		return err
	}

	return nil
}

func DeleteUserFeedLink(db *sqlx.DB, deleteUserFeedLinkPayload UserFeedLinkPayload) error {
	_, err := db.Exec(
		"DELETE FROM user_feeds WHERE user_id = $1 AND feed_id = $2",
		deleteUserFeedLinkPayload.UserId,
		deleteUserFeedLinkPayload.FeedId,
	)

	if err != nil {
		return err
	}

	return nil
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

		newItems = append(
			newItems,
			NewFeedContent{
				FeedId: feedId,
				Guid:   item.GUID,
				Title:  item.Title,
				ImgUrl: imgUrl,
				Link:   item.Link,
			},
		)
	}

	return newItems, nil
}

func UpdateUserContent(db *sqlx.DB, updateContentPayload UpdateUserContentPayload) error {
	tx, err := db.Beginx()

	if err != nil {
		return err
	}

	for _, updateRequest := range updateContentPayload.ContentToUpdate {
		newItemsToInsert, _ := getFeedContent(updateRequest.Url, updateRequest.FeedId)
		tx.NamedExec(
			`INSERT INTO feed_content (feed_id, "guid", title, img_url, "link")
			 VALUES (:feed_id, :guid, :title, :img_url, :link) ON CONFLICT DO NOTHING`,
			newItemsToInsert,
		)
	}

	err = tx.Commit()

	if err != nil {
		return err
	}

	return nil
}
