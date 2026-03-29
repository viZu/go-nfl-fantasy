# Gemini - NFL Fantasy Scraper

This project is a Go-based web scraper designed to extract historical league data from [NFL Fantasy](https://fantasy.nfl.com). It automates the collection of manager details, league settings, drafts, standings, matchups, playoffs, and trades across multiple seasons, exporting the results as structured JSON.

## Project Overview

- **Purpose**: To compile a comprehensive historical archive of a specific NFL Fantasy league, facilitating data migration or historical analysis.
- **Technologies**: 
    - **Go (Golang)**: Core language for performance and concurrency.
    - **Colly**: A powerful scraping framework for handling requests, concurrency, and HTML parsing.
    - **Goquery**: Integrated with Colly for jQuery-like DOM manipulation.

## Architecture

The project is modularized by entity to separate scraping concerns and ensure maintainability:

- **`main.go`**: Orchestrates the scraping workflow by invoking specialized modules.
- **`managers.go`**: Maps team IDs to manager names, user IDs, and team images.
- **`settings.go`**: Extracts league settings, including roster positions (with counts and optional maximums) and scoring rules mapped to Sleeper-compatible keys.
- **`drafts.go`**: Collects year-by-year draft results, including pick numbers, teams, and players.
- **`standings.go`**: Scrapes regular-season standings, handling divisions and detailed win/loss/points records.
- **`endstandings.go`**: Scrapes final season ranks and champion history.
- **`playoffs.go`**: Captures playoff brackets (Championship and Consolation), including seeds, scores, and round labels.
- **`matchups.go`**: Navigates weekly Game Centers to extract detailed player performance stats mapped to Sleeper variants.
- **`trades.go`**: Aggregates trade transactions, grouping multiple exchanges into single consolidated trade events.
- **`utils.go`**: Shared utility functions for mapping NFL positions and team names to Sleeper equivalents.
- **`collyhelper.go`**: Configures the Colly collector with authenticated sessions, rate limiting, and async rules.
- **`globals.go`**: Manages configuration parsed from `.env` (`LEAGUE_ID`, `START_YEAR`, `END_YEAR`, `NFL_COOKIE`).

## Output Data (JSON)

Scraped data is saved to the following files in the project root:

| File | Description |
| :--- | :--- |
| `managers-history.json` | Manager details, user IDs, and team metadata. |
| `settings-history.json` | Roster and scoring configurations per season. |
| `draft-history.json` | Chronological draft results. |
| `regular-season-standings-history.json` | Detailed standings including PF/PA and records. |
| `end-standings-history.json` | Final season rankings. |
| `playoff-history.json` | All playoff games across championship/consolation brackets. |
| `matchup-history.json` | Weekly matchups with individual player stats. |
| `trade-history.json` | Historical trade transactions between teams. |
| `end-roster-history.json` | End-of-season team rosters with starter statuses. |

## Building and Running

### Prerequisites
- Go 1.25 or higher.
- A valid `.env` file based on `.env.example` containing your `NFL_COOKIE`, `LEAGUE_ID`, and year range.

### Commands
- **Run the scraper**:
  ```bash
  go run .
  ```
- **Build the executable**:
  ```bash
  go build -o nfl-scraper
  ```

## Development Conventions

- **Concurrency**: Scraping is performed asynchronously using Colly's `Async(true)` mode. Rate limits and delays are configured globally to avoid blocking.
- **Thread Safety**: `sync.Mutex` is used across all modules to safely aggregate data from concurrent workers before sorting and writing to disk.
- **Data Mapping**: NFL positions and statistics are normalized to Sleeper.com naming conventions where possible via `utils.go`.
- **Surgical Extraction**: Prefers CSS selectors and Regular Expressions for precise data capture from complex NFL HTML structures.
- **Logging**: Beautified console output with emojis and progress tracking per year, including execution timers for each module.
