# sift-tui

Terminal TUI for [sift](../sift/) — browse and read RSS feeds from the same PostgreSQL database.

## Quick Install

Recommended (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/sift-tui/main/install.sh | bash
```

Or download a binary from [GitHub Releases](https://github.com/LFroesch/sift-tui/releases).

Or build from source:

```bash
make install
```

Command:

```bash
sift-tui
```
## Setup

Requires sift's DB to be running. Default connection: `postgres://postgres:postgres@localhost:5432/sift`

```bash
make run
# or
DATABASE_URL="postgres://..." ./sift-tui
```

## Keys

| View | Key | Action |
|------|-----|--------|
| Feeds | `j`/`k` | Navigate |
| | `enter` | Open feed |
| | `a` | Add feed (fetches immediately) |
| | `r` | Refresh selected feed |
| | `d` | Delete feed |
| | `n` | All unread |
| | `q` | Quit |
| Posts | `j`/`k` | Navigate |
| | `enter` | Read post (marks read) |
| | `m` | Toggle read |
| | `b` | Toggle bookmark |
| | `h`/`l` | Page |
| | `esc`/`q` | Back |
| Detail | `j`/`k` | Scroll |
| | `esc`/`q` | Back |

## Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) + Lipgloss
- PostgreSQL via `lib/pq` (shared with sift)
- [gofeed](https://github.com/mmcdole/gofeed) for RSS/Atom parsing
