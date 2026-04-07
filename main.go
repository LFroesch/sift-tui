package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/sift?sslmode=disable"
	}

	db, err := NewDB(dbURL)
	if err != nil {
		log.Fatalf("sift-tui: connect db: %v", err)
	}
	defer db.Close()

	feedRepo := NewFeedRepository(db)
	postRepo := NewPostRepository(db)
	fetcher := NewFeedFetcher(feedRepo)

	m := NewRootModel(feedRepo, postRepo, fetcher)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sift-tui: %v\n", err)
		os.Exit(1)
	}
}
