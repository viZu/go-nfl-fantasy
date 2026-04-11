package scraper

import (
	"fmt"
	"log"
	"strings"

	"github.com/gocolly/colly"
	"gonflfantasy/internal/config"
)

// ScrapeLeagueName visits the league home page and extracts the league name from the title.
func ScrapeLeagueName(cfg *config.Config) string {
	fmt.Println("[LEAGUE NAME] Starting league name scraper...")
	c := CreateColly(cfg, nil)
	var leagueName string

	c.OnHTML("head > title", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.Text)
		// Example Title: "Millers Home | NFL Fantasy"
		const suffix = " Home | NFL Fantasy"

		if strings.HasSuffix(title, suffix) {
			leagueName = strings.TrimSuffix(title, suffix)
		} else {
			// Fallback in case the title format changes
			leagueName = title
		}
	})

	targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s", cfg.LeagueID)

	if err := c.Visit(targetURL); err != nil {
		log.Printf("❌ [LEAGUE NAME] Error visiting league home: %v\n", err)
	}

	c.Wait()

	if leagueName != "" {
		fmt.Printf("\t✅ Successfully found league name: %s\n", leagueName)
	} else {
		fmt.Println("\t❌ Could not determine league name from title.")
	}

	return leagueName
}
