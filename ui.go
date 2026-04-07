package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ===== Styles (matches sb palette) =====

var (
	colorPrimary = lipgloss.Color("#5AF78E")
	colorAccent  = lipgloss.Color("#57C7FF")
	colorWarn    = lipgloss.Color("#FF6AC1")
	colorDim     = lipgloss.Color("#606060")
	colorYellow  = lipgloss.Color("#F3F99D")

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
	dimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	warnStyle   = lipgloss.NewStyle().Foreground(colorWarn)
	yellowStyle = lipgloss.NewStyle().Foreground(colorYellow)
)

// ===== View states =====

type viewState int

const (
	viewFeeds viewState = iota
	viewPosts
	viewDetail
	viewAddFeed
	viewUnread
)

// ===== Async messages =====

type feedAddedMsg struct {
	feed *Feed
	err  error
}

type feedRefreshedMsg struct {
	feedID string
	err    error
}

// ===== Model =====

type RootModel struct {
	view         viewState
	prevView     viewState
	feeds        []Feed
	posts        []Post
	selectedFeed int
	selectedPost int
	width        int
	height       int
	input        textinput.Model
	vp           viewport.Model
	vpReady      bool
	postsOffset  int
	postsLimit   int
	statusMsg    string
	statusIsErr  bool
	lastMsgTime  time.Time
	feedRepo     *FeedRepository
	postRepo     *PostRepository
	fetcher      *FeedFetcher
}

func NewRootModel(feedRepo *FeedRepository, postRepo *PostRepository, fetcher *FeedFetcher) *RootModel {
	ti := textinput.New()
	ti.Placeholder = "https://example.com/feed.xml"
	ti.Width = 60

	feeds, _ := feedRepo.GetAllFeeds()

	return &RootModel{
		view:       viewFeeds,
		feeds:      feeds,
		postsLimit: 50,
		feedRepo:   feedRepo,
		postRepo:   postRepo,
		fetcher:    fetcher,
		input:      ti,
	}
}

func (m RootModel) Init() tea.Cmd { return nil }

// ===== Update =====

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = msg.Height - 4
		}
		return m, nil

	case feedAddedMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
		} else {
			m.feeds = append(m.feeds, *msg.feed)
			m.setStatus("Added: " + msg.feed.Name)
		}
		return m, nil

	case feedRefreshedMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
		} else {
			m.setStatus("Refreshed")
			if m.view == viewPosts && len(m.feeds) > 0 &&
				m.feeds[m.selectedFeed].ID == msg.feedID {
				m.loadPosts()
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward non-key events to active component
	switch m.view {
	case viewAddFeed:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case viewDetail:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m RootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.view {
	case viewFeeds:
		return m.handleFeedKeys(msg)
	case viewPosts, viewUnread:
		return m.handlePostKeys(msg)
	case viewDetail:
		return m.handleDetailKeys(msg)
	case viewAddFeed:
		return m.handleAddFeedKeys(msg)
	}
	return m, nil
}

func (m RootModel) handleFeedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "j", "down":
		if m.selectedFeed < len(m.feeds)-1 {
			m.selectedFeed++
		}
	case "k", "up":
		if m.selectedFeed > 0 {
			m.selectedFeed--
		}
	case "enter":
		if len(m.feeds) > 0 {
			m.view = viewPosts
			m.postsOffset = 0
			m.selectedPost = 0
			m.loadPosts()
		}
	case "a":
		m.view = viewAddFeed
		m.input.SetValue("")
		return m, m.input.Focus()
	case "r":
		if len(m.feeds) > 0 {
			return m, m.refreshCmd(m.feeds[m.selectedFeed])
		}
	case "d":
		if len(m.feeds) > 0 {
			id := m.feeds[m.selectedFeed].ID
			m.feedRepo.DeleteFeed(id)
			m.feeds = append(m.feeds[:m.selectedFeed], m.feeds[m.selectedFeed+1:]...)
			if m.selectedFeed >= len(m.feeds) && m.selectedFeed > 0 {
				m.selectedFeed--
			}
			m.setStatus("Feed deleted")
		}
	case "n":
		m.view = viewUnread
		m.selectedPost = 0
		m.loadUnread()
	}
	return m, nil
}

func (m RootModel) handlePostKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.view = viewFeeds
		m.selectedPost = 0
	case "j", "down":
		if m.selectedPost < len(m.posts)-1 {
			m.selectedPost++
		}
	case "k", "up":
		if m.selectedPost > 0 {
			m.selectedPost--
		}
	case "enter":
		if len(m.posts) > 0 {
			m.openDetail()
		}
	case "m":
		if len(m.posts) > 0 {
			p := m.posts[m.selectedPost]
			m.postRepo.MarkAsRead(p.ID, !p.IsRead)
			m.posts[m.selectedPost].IsRead = !p.IsRead
		}
	case "b":
		if len(m.posts) > 0 {
			p := m.posts[m.selectedPost]
			m.postRepo.MarkAsBookmarked(p.ID, !p.IsBookmarked)
			m.posts[m.selectedPost].IsBookmarked = !p.IsBookmarked
		}
	case "l", "right":
		m.postsOffset += m.postsLimit
		m.loadPosts()
	case "h", "left":
		if m.postsOffset >= m.postsLimit {
			m.postsOffset -= m.postsLimit
			m.loadPosts()
		}
	}
	return m, nil
}

func (m RootModel) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.view = m.prevView
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m RootModel) handleAddFeedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewFeeds
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.input.Value())
		if url != "" {
			m.input.SetValue("")
			m.view = viewFeeds
			m.setStatus("Fetching…")
			return m, m.addFeedCmd(url)
		}
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ===== Commands =====

func (m RootModel) addFeedCmd(url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		feed, err := m.fetcher.FetchAndAddFeed(ctx, url, m.postRepo)
		return feedAddedMsg{feed, err}
	}
}

func (m RootModel) refreshCmd(feed Feed) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := m.fetcher.RefreshFeed(ctx, &feed, m.postRepo)
		return feedRefreshedMsg{feed.ID, err}
	}
}

// ===== Helpers =====

func (m *RootModel) loadPosts() {
	if len(m.feeds) == 0 {
		m.posts = nil
		return
	}
	posts, _ := m.postRepo.GetPostsByFeedID(
		m.feeds[m.selectedFeed].ID, m.postsLimit, m.postsOffset)
	m.posts = posts
}

func (m *RootModel) loadUnread() {
	posts, _ := m.postRepo.GetAllUnread()
	m.posts = posts
}

func (m *RootModel) openDetail() {
	if len(m.posts) == 0 {
		return
	}
	m.prevView = m.view
	p := m.posts[m.selectedPost]
	m.postRepo.MarkAsRead(p.ID, true)
	m.posts[m.selectedPost].IsRead = true

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(p.Title))
	sb.WriteString("\n\n")
	if p.PublishedAt != nil {
		sb.WriteString(dimStyle.Render(p.PublishedAt.Format("2006-01-02 15:04")))
		sb.WriteString("\n")
	}
	sb.WriteString(accentStyle.Render(p.URL))
	sb.WriteString("\n\n")
	if p.Description != "" {
		sb.WriteString(stripHTML(p.Description))
	} else {
		sb.WriteString(dimStyle.Render("(no description)"))
	}

	h := m.height - 4
	if h < 5 {
		h = 5
	}
	m.vp = viewport.New(m.width, h)
	m.vp.SetContent(sb.String())
	m.vpReady = true
	m.view = viewDetail
}

func (m *RootModel) setStatus(msg string) {
	m.statusMsg = msg
	m.statusIsErr = false
	m.lastMsgTime = time.Now()
}

func (m *RootModel) setError(msg string) {
	m.statusMsg = msg
	m.statusIsErr = true
	m.lastMsgTime = time.Now()
}

// ===== View =====

func (m RootModel) View() string {
	switch m.view {
	case viewFeeds:
		return m.viewFeeds()
	case viewPosts:
		return m.viewPosts()
	case viewDetail:
		return m.viewDetail()
	case viewAddFeed:
		return m.viewAddFeed()
	case viewUnread:
		return m.viewUnread()
	}
	return ""
}

func (m RootModel) viewFeeds() string {
	var lines []string
	lines = append(lines, titleStyle.Render("sift — RSS Feeds"))
	lines = append(lines, "")

	if len(m.feeds) == 0 {
		lines = append(lines, dimStyle.Render("  No feeds yet. Press 'a' to add one."))
	}

	for i, feed := range m.feeds {
		unread, _ := m.postRepo.GetUnreadCount(feed.ID)
		cursor := "  "
		if i == m.selectedFeed {
			cursor = accentStyle.Render("▶ ")
		}
		unreadStr := ""
		if unread > 0 {
			unreadStr = " " + yellowStyle.Render(fmt.Sprintf("(%d)", unread))
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", cursor, truncate(feed.Name, 50), unreadStr))
	}

	lines = append(lines, "")
	lines = append(lines, m.footer("a:add  r:refresh  d:delete  n:unread  enter:open  q:quit"))
	return strings.Join(lines, "\n")
}

func (m RootModel) viewPosts() string {
	feedName := ""
	if len(m.feeds) > 0 {
		feedName = m.feeds[m.selectedFeed].Name
	}

	var lines []string
	lines = append(lines, titleStyle.Render("sift — "+feedName))
	lines = append(lines, "")

	if len(m.posts) == 0 {
		lines = append(lines, dimStyle.Render("  No posts yet. Press 'r' to refresh this feed."))
	}

	for i, p := range m.posts {
		cursor := "  "
		if i == m.selectedPost {
			cursor = accentStyle.Render("▶ ")
		}
		readMark := " "
		if p.IsRead {
			readMark = dimStyle.Render("✓")
		}
		bookMark := " "
		if p.IsBookmarked {
			bookMark = yellowStyle.Render("★")
		}
		date := "     "
		if p.PublishedAt != nil {
			date = dimStyle.Render(p.PublishedAt.Format("01-02"))
		}
		title := truncate(p.Title, m.width-18)
		if p.IsRead {
			title = dimStyle.Render(title)
		}
		lines = append(lines, fmt.Sprintf("%s[%s%s] %s  %s", cursor, readMark, bookMark, title, date))
	}

	lines = append(lines, "")
	lines = append(lines, m.footer("m:read  b:bookmark  h/l:page  enter:open  esc:back"))
	return strings.Join(lines, "\n")
}

func (m RootModel) viewDetail() string {
	if !m.vpReady {
		return dimStyle.Render("loading...")
	}
	return m.vp.View() + "\n" + m.footer("j/k:scroll  esc:back")
}

func (m RootModel) viewAddFeed() string {
	return strings.Join([]string{
		titleStyle.Render("sift — Add Feed"),
		"",
		"Feed URL:",
		m.input.View(),
		"",
		m.footer("enter:add  esc:cancel"),
	}, "\n")
}

func (m RootModel) viewUnread() string {
	var lines []string
	lines = append(lines, titleStyle.Render(fmt.Sprintf("sift — Unread (%d)", len(m.posts))))
	lines = append(lines, "")

	if len(m.posts) == 0 {
		lines = append(lines, dimStyle.Render("  Nothing unread."))
	}

	for i, p := range m.posts {
		cursor := "  "
		if i == m.selectedPost {
			cursor = accentStyle.Render("▶ ")
		}
		bookMark := " "
		if p.IsBookmarked {
			bookMark = yellowStyle.Render("★")
		}
		date := "     "
		if p.PublishedAt != nil {
			date = dimStyle.Render(p.PublishedAt.Format("01-02"))
		}
		feed := dimStyle.Render("[" + truncate(m.feedName(p.FeedID), 18) + "]")
		title := truncate(p.Title, m.width-30)
		lines = append(lines, fmt.Sprintf("%s[%s] %s  %s  %s", cursor, bookMark, title, date, feed))
	}

	lines = append(lines, "")
	lines = append(lines, m.footer("m:read  b:bookmark  enter:open  esc:back"))
	return strings.Join(lines, "\n")
}

// ===== Render helpers =====

func (m RootModel) footer(keys string) string {
	status := ""
	if m.statusMsg != "" && time.Since(m.lastMsgTime) < 4*time.Second {
		if m.statusIsErr {
			status = " | " + warnStyle.Render(m.statusMsg)
		} else {
			status = " | " + yellowStyle.Render(m.statusMsg)
		}
	}
	return dimStyle.Render(keys) + status
}

func (m RootModel) feedName(feedID string) string {
	for _, f := range m.feeds {
		if f.ID == feedID {
			return f.Name
		}
	}
	return "?"
}

func truncate(s string, n int) string {
	if n <= 3 || len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func stripHTML(s string) string {
	var out strings.Builder
	inTag := false
	for _, c := range s {
		switch {
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
			out.WriteRune(' ')
		case !inTag:
			out.WriteRune(c)
		}
	}
	// Collapse multiple spaces/newlines
	result := strings.TrimSpace(out.String())
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return result
}
