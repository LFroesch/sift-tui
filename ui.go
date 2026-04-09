package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

type viewState int

const (
	viewFeeds viewState = iota
	viewPosts
	viewDetail
	viewUnread
)

type feedsReloadedMsg struct {
	feeds    []Feed
	newPosts int
	fetched  bool
	err      error
}

type RootModel struct {
	view         viewState
	prevView     viewState
	feeds        []Feed
	posts        []Post
	selectedFeed int
	selectedPost int
	width        int
	height       int
	vp           viewport.Model
	vpReady      bool
	postsOffset  int
	postsLimit   int
	statusMsg    string
	statusIsErr  bool
	lastMsgTime  time.Time
	feedRepo     *FeedRepository
	postRepo     *PostRepository
}

func NewRootModel(feedRepo *FeedRepository, postRepo *PostRepository) *RootModel {
	feeds, _ := feedRepo.GetAllFeeds()
	return &RootModel{
		view:       viewFeeds,
		feeds:      feeds,
		postsLimit: 50,
		feedRepo:   feedRepo,
		postRepo:   postRepo,
	}
}

func (m RootModel) Init() tea.Cmd { return nil }

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

	case feedsReloadedMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
			return m, nil
		}
		m.feeds = msg.feeds
		if m.selectedFeed >= len(m.feeds) && len(m.feeds) > 0 {
			m.selectedFeed = len(m.feeds) - 1
		}
		if len(m.feeds) == 0 {
			m.selectedFeed = 0
			m.posts = nil
		}
		if msg.fetched {
			if msg.newPosts > 0 {
				m.setStatus(fmt.Sprintf("Fetched +%d new", msg.newPosts))
			} else {
				m.setStatus("Fetched: up to date")
			}
		} else {
			m.setStatus("Refreshed feeds")
		}
		if m.view == viewPosts {
			m.loadPosts()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.view == viewDetail {
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
	case "r":
		return m, m.reloadFeedsCmd()
	case "f":
		m.setStatus("Fetching feeds…")
		return m, m.fetchAllCmd()
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
			if err := m.postRepo.MarkAsRead(p.ID, !p.IsRead); err != nil {
				m.setError(err.Error())
				return m, nil
			}
			m.posts[m.selectedPost].IsRead = !p.IsRead
		}
	case "b":
		if len(m.posts) > 0 {
			p := m.posts[m.selectedPost]
			if err := m.postRepo.MarkAsBookmarked(p.ID, !p.IsBookmarked); err != nil {
				m.setError(err.Error())
				return m, nil
			}
			m.posts[m.selectedPost].IsBookmarked = !p.IsBookmarked
		}
	case "l", "right":
		if m.view == viewPosts {
			m.postsOffset += m.postsLimit
			m.loadPosts()
		}
	case "h", "left":
		if m.view == viewPosts && m.postsOffset >= m.postsLimit {
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

func (m RootModel) reloadFeedsCmd() tea.Cmd {
	return func() tea.Msg {
		feeds, err := m.feedRepo.GetAllFeeds()
		return feedsReloadedMsg{feeds: feeds, fetched: false, err: err}
	}
}

func (m RootModel) fetchAllCmd() tea.Cmd {
	return func() tea.Msg {
		newPosts, err := m.feedRepo.FetchAllFeeds()
		if err != nil {
			return feedsReloadedMsg{err: err, fetched: true}
		}
		feeds, err := m.feedRepo.GetAllFeeds()
		return feedsReloadedMsg{feeds: feeds, newPosts: newPosts, fetched: true, err: err}
	}
}

func (m *RootModel) loadPosts() {
	if len(m.feeds) == 0 {
		m.posts = nil
		return
	}
	posts, err := m.postRepo.GetPostsByFeedID(
		m.feeds[m.selectedFeed].ID, m.postsLimit, m.postsOffset)
	if err != nil {
		m.setError(err.Error())
		return
	}
	m.posts = posts
}

func (m *RootModel) loadUnread() {
	posts, err := m.postRepo.GetAllUnread()
	if err != nil {
		m.setError(err.Error())
		return
	}
	m.posts = posts
}

func (m *RootModel) openDetail() {
	if len(m.posts) == 0 {
		return
	}
	m.prevView = m.view
	p := m.posts[m.selectedPost]
	if err := m.postRepo.MarkAsRead(p.ID, true); err != nil {
		m.setError(err.Error())
	} else {
		m.posts[m.selectedPost].IsRead = true
	}

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

func (m RootModel) View() string {
	switch m.view {
	case viewFeeds:
		return m.viewFeeds()
	case viewPosts:
		return m.viewPosts()
	case viewDetail:
		return m.viewDetail()
	case viewUnread:
		return m.viewUnread()
	}
	return ""
}

func (m RootModel) viewFeeds() string {
	var lines []string
	lines = append(lines, titleStyle.Render("sift — Feeds"))
	lines = append(lines, dimStyle.Render("API mode"))
	lines = append(lines, "")

	if len(m.feeds) == 0 {
		lines = append(lines, dimStyle.Render("  No feeds yet. Add feeds in sift web UI."))
	}

	for i, feed := range m.feeds {
		cursor := "  "
		if i == m.selectedFeed {
			cursor = accentStyle.Render("▶ ")
		}
		groupSuffix := ""
		if len(feed.Groups) > 0 {
			groupSuffix = dimStyle.Render(fmt.Sprintf(" [%d groups]", len(feed.Groups)))
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", cursor, truncate(feed.Name, 64), groupSuffix))
	}

	lines = append(lines, "")
	lines = append(lines, m.footer("r:reload  f:fetch-all  n:unread  enter:open  q:quit"))
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
		lines = append(lines, dimStyle.Render("  No posts found for this feed."))
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
	result := strings.TrimSpace(out.String())
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return result
}
