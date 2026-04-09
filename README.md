# sift-tui

Terminal read mode for `sift`.

`sift-tui` now talks to `sift` over HTTP (`/api`) instead of connecting to Postgres directly, so behavior stays aligned with the web app.

## Quick Install

Recommended (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/sift-tui/main/install.sh | bash
```

Or build from source:

```bash
make install
```

## Setup

Run `sift` backend first (default: `http://localhost:5005`).

```bash
# optional if running sift somewhere else
export SIFT_API_URL="http://localhost:5005"

sift-tui
```

Notes:
- `SIFT_API_URL` can be either `http://host:port` or `http://host:port/api`.
- Feed creation/deletion is intentionally handled in the web app; TUI is focused on reading.

## Keys

| View | Key | Action |
|------|-----|--------|
| Feeds | `j`/`k` | Navigate |
| | `enter` | Open feed posts |
| | `r` | Reload feeds |
| | `f` | Fetch all feeds |
| | `n` | Unread view |
| | `q` | Quit |
| Posts | `j`/`k` | Navigate |
| | `enter` | Open detail (marks read) |
| | `m` | Toggle read |
| | `b` | Toggle bookmark |
| | `h`/`l` | Page |
| | `esc`/`q` | Back |
| Unread | `j`/`k` | Navigate |
| | `enter` | Open detail |
| | `m` | Toggle read |
| | `b` | Toggle bookmark |
| | `esc`/`q` | Back |
| Detail | `j`/`k` | Scroll |
| | `esc`/`q` | Back |

## Stack

- Bubble Tea + Lipgloss
- `sift` HTTP API
