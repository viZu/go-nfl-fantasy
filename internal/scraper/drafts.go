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

	// Sort by Year (ascending) and then by PickNumber (ascending)
	sort.Slice(allPicks, func(i, j int) bool {
		if allPicks[i].Year != allPicks[j].Year {
			return allPicks[i].Year < allPicks[j].Year
		}
		return allPicks[i].PickNumber < allPicks[j].PickNumber
	})

	// Write to JSON file
	file, err := os.Create("draft-history.json")
	if err != nil {
		log.Printf("Error creating draft-history.json: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allPicks); err != nil {
		log.Printf("❌ [DRAFTS] Error encoding draft history to JSON: %v\n", err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d picks to draft-history.json (took %s)\n", len(allPicks), time.Since(startTime))
	}
}
