package scraper

import (
	"encoding/json"
	"fmt"
	"gonflfantasy/internal/config"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

type DraftPick struct {
	Year       int    `json:"year"`
	Round      int    `json:"round"`
	PickNumber int    `json:"pick"`
	TeamID     string `json:"teamId"`
	TeamName   string `json:"teamName"`
	PlayerID   string `json:"playerId"`
	PlayerName string `json:"playerName"`
}

func ScrapeDrafts(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[DRAFTS] Starting draft results scraper...")
	c := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	roundRegex := regexp.MustCompile(`Round (\d+)`)

	allPicks := make([]DraftPick, 0)
	var mu sync.Mutex

	c.OnHTML(".results .wrap", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)
		roundStr := e.ChildText("h4")
		round := 0
		if matches := roundRegex.FindStringSubmatch(roundStr); len(matches) > 1 {
			round, _ = strconv.Atoi(matches[1])
		}

		e.ForEach("ul li", func(_ int, el *colly.HTMLElement) {
			// Pick Number
			pickStr := el.ChildText(".count")
			pickNumber, _ := strconv.Atoi(strings.TrimSuffix(pickStr, "."))

			// Player Name and ID
			playerName := el.ChildText(".playerName")
			playerID := ""
			playerClass := el.ChildAttr(".playerName", "class")
			if matches := playerIDRegex.FindStringSubmatch(playerClass); len(matches) > 1 {
				playerID = matches[1]
			}

			// Team Name and ID
			teamName := el.ChildText(".teamName")
			teamID := ""
			teamClass := el.ChildAttr(".teamName", "class")
			if matches := teamIDRegex.FindStringSubmatch(teamClass); len(matches) > 1 {
				teamID = matches[1]
			}

			if playerName != "" {

				pick := DraftPick{
					Year:       year,
					Round:      round,
					PickNumber: pickNumber,
					TeamID:     teamID,
					TeamName:   teamName,
					PlayerID:   playerID,
					PlayerName: playerName,
				}

				// Safely append to the shared slice across concurrent workers
				mu.Lock()
				allPicks = append(allPicks, pick)
				mu.Unlock()
			}
		})
	})

	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		// Updated URL with query parameters to ensure all rounds are returned
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/draftresults?draftResultsDetail=0&draftResultsTab=round&draftResultsType=results", cfg.LeagueID, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Println("Error visiting page:", err)
		}
	}

	c.Wait()

	// Group drafts by year
	draftsByYear := groupDraftsByYear(allPicks)
	years := getSortedDraftYears(draftsByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writeDraftYear(year, draftsByYear[year], exportDir)
	}
	fmt.Printf("\t✅ Completed draft history scraping (took %s)\n", time.Since(startTime))
}

func groupDraftsByYear(allPicks []DraftPick) map[int][]DraftPick {
	draftsByYear := make(map[int][]DraftPick)
	for _, pick := range allPicks {
		draftsByYear[pick.Year] = append(draftsByYear[pick.Year], pick)
	}
	return draftsByYear
}

func getSortedDraftYears(draftsByYear map[int][]DraftPick) []int {
	var years []int
	for year := range draftsByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writeDraftYear(year int, yearPicks []DraftPick, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	fileName := "draft-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [DRAFTS] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearPicks); err != nil {
		log.Printf("❌ [DRAFTS] Error encoding draft history to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d picks to %d/%s\n", len(yearPicks), year, fileName)
	}
}
