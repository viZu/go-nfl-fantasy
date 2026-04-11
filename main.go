package main

import (
	"gonflfantasy/internal/config"
	"gonflfantasy/internal/scraper"
)

func main() {
	cfg := config.Load()

	// Scrape and set the league name first
	cfg.LeagueName = scraper.ScrapeLeagueName(cfg)

	scraper.ScrapeManagers(cfg)
	scraper.ScrapeSettings(cfg)
	scraper.ScrapeDrafts(cfg)
	scraper.ScrapeRosters(cfg)
	scraper.ScrapeStandings(cfg)
	scraper.ScrapePlayoffs(cfg)
	scraper.ScrapeEndStandings(cfg)
	scraper.ScrapeMatchups(cfg)
	scraper.ScrapeTrades(cfg)
}
