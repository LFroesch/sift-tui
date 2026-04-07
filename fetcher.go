package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type FeedFetcher struct {
	repo   *FeedRepository
	parser *gofeed.Parser
}

func NewFeedFetcher(repo *FeedRepository) *FeedFetcher {
	return &FeedFetcher{repo: repo, parser: gofeed.NewParser()}
}

func (ff *FeedFetcher) FetchAndAddFeed(ctx context.Context, url string, postRepo *PostRepository) (*Feed, error) {
	parsed, err := ff.parser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	name := parsed.Title
	if name == "" {
		name = url
	}

	feed, err := ff.repo.CreateFeed(name, url)
	if err != nil {
		return nil, fmt.Errorf("create feed: %w", err)
	}

	ff.insertPosts(feed.ID, parsed.Items, postRepo)

	now := time.Now()
	feed.LastFetchedAt = &now
	ff.repo.UpdateLastFetched(feed.ID)
	return feed, nil
}

func (ff *FeedFetcher) RefreshFeed(ctx context.Context, feed *Feed, postRepo *PostRepository) error {
	parsed, err := ff.parser.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return fmt.Errorf("parse feed: %w", err)
	}
	ff.insertPosts(feed.ID, parsed.Items, postRepo)
	return ff.repo.UpdateLastFetched(feed.ID)
}

func (ff *FeedFetcher) insertPosts(feedID string, items []*gofeed.Item, postRepo *PostRepository) {
	for _, item := range items {
		if item.Link == "" {
			continue
		}
		p := &Post{
			FeedID:      feedID,
			Title:       item.Title,
			URL:         item.Link,
			Description: item.Description,
		}
		if item.PublishedParsed != nil {
			p.PublishedAt = item.PublishedParsed
		}
		postRepo.UpsertPost(p)
	}
}
