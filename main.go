package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	apiURL := os.Getenv("SIFT_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:5005"
	}
	apiURL = strings.TrimRight(apiURL, "/")
	if !strings.HasSuffix(apiURL, "/api") {
		apiURL += "/api"
	}

	client := NewAPIClient(apiURL)
	feedRepo := NewFeedRepository(client)
	postRepo := NewPostRepository(client)

	m := NewRootModel(feedRepo, postRepo)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sift-tui: %v\n", err)
		os.Exit(1)
	}
}
