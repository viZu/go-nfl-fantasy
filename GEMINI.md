# Gemini - NFL Fantasy Scraper

This project is a Go-based web scraper designed to extract historical league data from [NFL Fantasy](https://fantasy.nfl.com). It automates the collection of manager details, weekly matchups, and player performance records across multiple seasons.

## Project Overview

- **Purpose**: To compile a comprehensive historical archive of a specific NFL Fantasy league, including standings, matchups, and rosters.
- **Technologies**: 
    - **Go (Golang)**: Core language for performance and concurrency.
    - **Colly**: A powerful scraping framework for handling requests, concurrency, and HTML parsing.
    - **Goquery**: Integrated with Colly for jQuery-like DOM manipulation.

## Architecture

The project is structured into functional modules to separate scraping concerns:

- **`main.go`**: The entry point that orchestrates the scraping workflow.
- **`managers.go`**: Scrapes the league's "History -> Owners" page to map team IDs to manager names and user IDs.
- **`matchups.go`**: Navigates through the league schedule and "Game Center" pages to extract weekly scores and detailed player stats for every matchup.
- **`collyhelper.go`**: A utility module to initialize a configured Colly collector with appropriate User-Agents, cookies for authentication, and rate-limiting rules.
- **`globals.go`**: Contains global configuration such as `leagueId`, `startYear`, `endYear`, and the `nflCookie` required for accessing private league data.

## Building and Running

### Prerequisites
- Go 1.25 or higher.
- A valid `NFL_COOKIE` in `.env` (retrieved from a logged-in browser session).

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

- **Concurrency**: Scraping is performed asynchronously using Colly's `Async(true)` mode. Rate limits and delays are configured in `collyhelper.go` and `matchups.go` to avoid being blocked.
- **Data Extraction**: Prefers CSS selectors via `OnHTML`. Regular expressions are used for surgical extraction of IDs from attributes and URLs.
- **Context Management**: Uses `colly.Context` to propagate metadata (e.g., Year, Week, Matchup ID) through the asynchronous request pipeline.
- **Error Handling**: Scrapers include error handlers to log failed requests without halting the entire process.

## TODOs / Future Improvements
- [ ] Implement data persistence (e.g., export to JSON, CSV, or a SQL database).
- [ ] Add unit tests for the HTML parsing logic (`parsePlayerRow`, etc.).
- [ ] Add support for scraping Draft Results and Season End Rosters as mentioned in `main.go`.
