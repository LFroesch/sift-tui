package main

import (
	"database/sql"
	"time"
)

// ===== Types =====

type Feed struct {
	ID            string
	Name          string
	URL           string
	LastFetchedAt *time.Time
}

type Post struct {
	ID           string
	FeedID       string
	Title        string
	URL          string
	Description  string
	PublishedAt  *time.Time
	IsRead       bool
	IsBookmarked bool
}

// ===== FeedRepository =====

type FeedRepository struct{ db *DB }

func NewFeedRepository(db *DB) *FeedRepository { return &FeedRepository{db} }

func (r *FeedRepository) GetAllFeeds() ([]Feed, error) {
	rows, err := r.db.Query(
		`SELECT id, name, url, last_fetched_at FROM feeds ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []Feed
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.ID, &f.Name, &f.URL, &f.LastFetchedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func (r *FeedRepository) CreateFeed(name, url string) (*Feed, error) {
	var f Feed
	err := r.db.QueryRow(
		`INSERT INTO feeds (name, url) VALUES ($1, $2)
		 RETURNING id, name, url, last_fetched_at`,
		name, url,
	).Scan(&f.ID, &f.Name, &f.URL, &f.LastFetchedAt)
	return &f, err
}

func (r *FeedRepository) DeleteFeed(id string) error {
	_, err := r.db.Exec(`DELETE FROM feeds WHERE id = $1`, id)
	return err
}

func (r *FeedRepository) UpdateLastFetched(id string) error {
	_, err := r.db.Exec(
		`UPDATE feeds SET last_fetched_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

// ===== PostRepository =====

type PostRepository struct{ db *DB }

func NewPostRepository(db *DB) *PostRepository { return &PostRepository{db} }

func (r *PostRepository) GetPostsByFeedID(feedID string, limit, offset int) ([]Post, error) {
	rows, err := r.db.Query(
		`SELECT id, feed_id, title, url, COALESCE(description, ''), published_at, is_read, is_bookmarked
		 FROM posts WHERE feed_id = $1
		 ORDER BY COALESCE(published_at, created_at) DESC
		 LIMIT $2 OFFSET $3`,
		feedID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (r *PostRepository) GetAllUnread() ([]Post, error) {
	rows, err := r.db.Query(
		`SELECT id, feed_id, title, url, COALESCE(description, ''), published_at, is_read, is_bookmarked
		 FROM posts WHERE is_read = FALSE
		 ORDER BY COALESCE(published_at, created_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (r *PostRepository) MarkAsRead(id string, isRead bool) error {
	_, err := r.db.Exec(
		`UPDATE posts SET is_read = $1, updated_at = NOW() WHERE id = $2`, isRead, id)
	return err
}

func (r *PostRepository) MarkAsBookmarked(id string, isBookmarked bool) error {
	_, err := r.db.Exec(
		`UPDATE posts SET is_bookmarked = $1, updated_at = NOW() WHERE id = $2`, isBookmarked, id)
	return err
}

func (r *PostRepository) GetUnreadCount(feedID string) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM posts WHERE feed_id = $1 AND is_read = FALSE`, feedID,
	).Scan(&count)
	return count, err
}

func (r *PostRepository) UpsertPost(p *Post) error {
	desc := sql.NullString{}
	if p.Description != "" {
		desc = sql.NullString{String: p.Description, Valid: true}
	}
	_, err := r.db.Exec(
		`INSERT INTO posts (title, url, description, published_at, feed_id, is_read, is_bookmarked)
		 VALUES ($1, $2, $3, $4, $5, FALSE, FALSE)
		 ON CONFLICT (url) DO NOTHING`,
		p.Title, p.URL, desc, p.PublishedAt, p.FeedID,
	)
	return err
}

func scanPosts(rows *sql.Rows) ([]Post, error) {
	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.FeedID, &p.Title, &p.URL, &p.Description,
			&p.PublishedAt, &p.IsRead, &p.IsBookmarked); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
