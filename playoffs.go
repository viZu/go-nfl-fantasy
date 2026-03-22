package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

type PlayoffMatchup struct {
	Year        int
	Week        int
	RoundName   string
	BracketType string
	Team1ID     string
	Team1Points float32
	Team2ID     string
	Team2Points float32
	WinnerID    string
}

// Local regex for playoff week labels
var playoffWeekRegex = regexp.MustCompile(`Week (\d+)`)

func scrapePlayoffs() {
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	c.OnHTML("ul.playoffContent > li", func(e *colly.HTMLElement) {
		bracketType := e.Request.Ctx.Get("bracketType")

		// Extract Week Number from the round header
		weekStr := e.ChildText("h4")
		week := 0
		if matches := playoffWeekRegex.FindStringSubmatch(weekStr); len(matches) > 1 {
			week, _ = strconv.Atoi(matches[1])
		}

		// Iterate over matchups in this round
		e.ForEach("ul > li[class*='pg-']", func(_ int, el *colly.HTMLElement) {
			roundLabel := el.ChildText("h5") // e.g., "Quarterfinal", "Fantasy Super Bowl", "3rd Place Game"

			// Team 1 Extraction
			team1ID := ""
			t1Class := el.ChildAttr(".teamWrap-1 .teamName", "class")
			if matches := teamIDRegex.FindStringSubmatch(t1Class); len(matches) > 1 {
				team1ID = matches[1]
			}
			t1PointsStr := el.ChildText(".teamWrap-1 .teamTotal")
			t1Points, _ := strconv.ParseFloat(strings.TrimSpace(t1PointsStr), 32)

			// Team 2 Extraction
			team2ID := ""
			t2Class := el.ChildAttr(".teamWrap-2 .teamName", "class")
			if matches := teamIDRegex.FindStringSubmatch(t2Class); len(matches) > 1 {
				team2ID = matches[1]
			}
			t2PointsStr := el.ChildText(".teamWrap-2 .teamTotal")
			t2Points, _ := strconv.ParseFloat(strings.TrimSpace(t2PointsStr), 32)

			// Determine Winner
			winnerID := ""
			if t1Points > t2Points {
				winnerID = team1ID
			} else if t2Points > t1Points {
				winnerID = team2ID
			}

			if team1ID != "" && team2ID != "" {
				// Matchup ID info (as requested in objective)
				// Format: { "week": week, "teamId": team1ID }
				fmt.Printf("    [Playoffs] %-12s | Week %d | %-20s | T1: %-2s (%6.2f) vs T2: %-2s (%6.2f) | Winner: %s\n",
					bracketType, week, roundLabel, team1ID, t1Points, team2ID, t2Points, winnerID)
			}
		})
	})

	for year := startYear; year <= endYear; year++ {
		// Championship Bracket
		champURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/playoffs?bracketType=championship&standingsTab=playoffs", leagueId, year)
		ctxChamp := colly.NewContext()
		ctxChamp.Put("year", year)
		ctxChamp.Put("bracketType", "Championship")

		fmt.Printf("Scraping Championship playoffs for %d...\n", year)
		err := c.Request("GET", champURL, nil, ctxChamp, nil)
		if err != nil {
			log.Printf("Error requesting Championship playoffs for %d: %v", year, err)
		}

		// Consolation Bracket
		consURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/playoffs?bracketType=consolation&standingsTab=playoffs", leagueId, year)
		ctxCons := colly.NewContext()
		ctxCons.Put("year", year)
		ctxCons.Put("bracketType", "Consolation")

		fmt.Printf("Scraping Consolation playoffs for %d...\n", year)
		err = c.Request("GET", consURL, nil, ctxCons, nil)
		if err != nil {
			log.Printf("Error requesting Consolation playoffs for %d: %v", year, err)
		}
	}

	c.Wait()
}
