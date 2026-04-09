package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type APIClient struct {
	baseURL string
	http    *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type apiError struct {
	Error string `json:"error"`
}

func (c *APIClient) doJSON(method, path string, query url.Values, body any, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("api %s %s: %s", method, path, apiErr.Error)
		}
		return fmt.Errorf("api %s %s: status %d", method, path, resp.StatusCode)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ===== Types =====

type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Feed struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	LastFetchedAt *time.Time `json:"last_fetched_at"`
	Groups        []Group    `json:"groups"`
}

type Post struct {
	ID           string     `json:"id"`
	FeedID       string     `json:"feed_id"`
	FeedName     string     `json:"feed_name"`
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Description  string     `json:"description"`
	PublishedAt  *time.Time `json:"published_at"`
	IsRead       bool       `json:"is_read"`
	IsBookmarked bool       `json:"is_bookmarked"`
}

type postsResponse struct {
	Posts   []Post `json:"posts"`
	HasMore bool   `json:"hasMore"`
}

type fetchResponse struct {
	NewPosts int `json:"newPosts"`
}

// ===== FeedRepository =====

type FeedRepository struct{ api *APIClient }

func NewFeedRepository(api *APIClient) *FeedRepository { return &FeedRepository{api: api} }

func (r *FeedRepository) GetAllFeeds() ([]Feed, error) {
	var feeds []Feed
	err := r.api.doJSON(http.MethodGet, "/feeds", nil, nil, &feeds)
	return feeds, err
}

func (r *FeedRepository) FetchAllFeeds() (int, error) {
	var resp fetchResponse
	err := r.api.doJSON(http.MethodPost, "/fetch", nil, nil, &resp)
	return resp.NewPosts, err
}

// Legacy methods kept so old helpers compile.
func (r *FeedRepository) CreateFeed(_, _ string) (*Feed, error) {
	return nil, fmt.Errorf("not supported in read mode")
}

func (r *FeedRepository) DeleteFeed(_ string) error {
	return fmt.Errorf("not supported in read mode")
}

func (r *FeedRepository) UpdateLastFetched(_ string) error { return nil }

// ===== PostRepository =====

type PostRepository struct{ api *APIClient }

func NewPostRepository(api *APIClient) *PostRepository { return &PostRepository{api: api} }

func (r *PostRepository) GetPostsByFeedID(feedID string, limit, offset int) ([]Post, error) {
	q := url.Values{}
	q.Set("feed_id", feedID)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp postsResponse
	if err := r.api.doJSON(http.MethodGet, "/posts", q, nil, &resp); err != nil {
		return nil, err
	}
	return normalizePosts(resp.Posts), nil
}

func (r *PostRepository) GetAllUnread() ([]Post, error) {
	limit := 200
	offset := 0
	var out []Post

	for {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))

		var resp postsResponse
		if err := r.api.doJSON(http.MethodGet, "/posts", q, nil, &resp); err != nil {
			return nil, err
		}

		for _, p := range normalizePosts(resp.Posts) {
			if !p.IsRead {
				out = append(out, p)
			}
		}

		if !resp.HasMore || len(resp.Posts) == 0 || offset > 5000 {
			break
		}
		offset += limit
	}
	return out, nil
}

func (r *PostRepository) MarkAsRead(id string, isRead bool) error {
	path := "/posts/" + id + "/read"
	if !isRead {
		path = "/posts/" + id + "/unread"
	}
	return r.api.doJSON(http.MethodPatch, path, nil, nil, nil)
}

func (r *PostRepository) MarkAsBookmarked(id string, _ bool) error {
	path := "/posts/" + id + "/bookmark"
	return r.api.doJSON(http.MethodPatch, path, nil, nil, nil)
}

func (r *PostRepository) GetUnreadCount(feedID string) (int, error) {
	posts, err := r.GetPostsByFeedID(feedID, 200, 0)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, p := range posts {
		if !p.IsRead {
			count++
		}
	}
	return count, nil
}

// Legacy method kept so old helpers compile.
func (r *PostRepository) UpsertPost(_ *Post) error { return nil }

func normalizePosts(posts []Post) []Post {
	for i := range posts {
		if posts[i].Description == "null" {
			posts[i].Description = ""
		}
	}
	return posts
}
