# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build

# Run (all files must be specified or use build first)
go run main.go collyhelper.go globals.go managers.go matchups.go

# Sync dependencies
go mod tidy
```

There are no tests in this project.

## Architecture

This is a Go web scraper for a private NFL Fantasy Football league on fantasy.nfl.com, using the [Colly](https://github.com/gocolly/colly) framework.

**Authentication:** A session cookie in `globals.go` is injected into every request via `createColly()` in `collyhelper.go`. The cookie must be kept valid for scraping to work.

**Two main scrapers:**

- `scrapeManagers()` (`managers.go`) — loops years 2010–2024, visits the `/owners` page for each year, and returns a `map[TeamKey]Manager` lookup table used to correlate team IDs to manager names.
- `scrapeMatchups()` (`matchups.go`) — currently hardcoded to `endYear` (2024). Uses two chained Colly collectors: one visits schedule pages to discover matchup URLs, a second visits each game center to extract per-team rosters and scoring. Runs with 32 parallel workers.

**Data flow:** Scraped data lands in in-memory structs (`Manager`, `Matchup`, `Player`). There is currently no persistence layer — results are only printed to stdout.

**Rate limiting:** `createColly()` sets a 1–1.5s delay globally. The matchup collector uses a tighter 200–700ms delay with parallelism capped at 32 to avoid blocks.