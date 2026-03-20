package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

type DraftPick struct {
	Year       int
	Round      int
	PickNumber int
	PlayerID   string
	PlayerName string
	TeamID     string
	TeamName   string
}

func scrapeDrafts() {
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	roundRegex := regexp.MustCompile(`Round (\d+)`)

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
				fmt.Printf("    [Draft] Year: %d | Rnd: %-2d | Pick: %-3d | Team ID: %-2s | Team: %-20s | Player: %s (%s)\n",
					year, round, pickNumber, teamID, teamName, playerName, playerID)
			}
		})
	})

	for year := startYear; year <= endYear; year++ {
		// Updated URL with query parameters to ensure all rounds are returned
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/draftresults?draftResultsDetail=0&draftResultsTab=round&draftResultsType=results", leagueId, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		fmt.Printf("Scraping draft results for %d...\n", year)
		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Println("Error visiting page:", err)
		}
	}

	c.Wait()
}
