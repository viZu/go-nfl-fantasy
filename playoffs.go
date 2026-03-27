package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gocolly/colly"
)

type PlayoffGame struct {
	Year        int     `json:"year"`
	Week        int     `json:"week"`
	Round       int     `json:"round"`
	RoundLabel  string  `json:"roundLabel"`
	BracketType string  `json:"bracketType"`
	Team1       string  `json:"team1"`
	Team2       string  `json:"team2"`
	Team1Points float32 `json:"team1Points"`
	Team2Points float32 `json:"team2Points"`
	Winner      string  `json:"winner"`
}

// Local regex for playoff week labels
var playoffWeekRegex = regexp.MustCompile(`Week (\d+)`)

func scrapePlayoffs() {
	fmt.Println("Scraping playoffs...")

	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	var allGames []PlayoffGame
	var mu sync.Mutex

	c.OnHTML("ul.playoffContent > li", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)
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
				game := PlayoffGame{
					Year:        year,
					Week:        week,
					Round:       getPlayoffRoundNumber(roundLabel),
					RoundLabel:  roundLabel,
					BracketType: bracketType,
					Team1:       team1ID,
					Team2:       team2ID,
					Team1Points: float32(t1Points),
					Team2Points: float32(t2Points),
					Winner:      winnerID,
				}

				mu.Lock()
				allGames = append(allGames, game)
				mu.Unlock()
			}
		})
	})

	for year := startYear; year <= endYear; year++ {
		// Championship Bracket
		champURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/playoffs?bracketType=championship&standingsTab=playoffs", leagueId, year)
		ctxChamp := colly.NewContext()
		ctxChamp.Put("year", year)
		ctxChamp.Put("bracketType", "Championship")

		err := c.Request("GET", champURL, nil, ctxChamp, nil)
		if err != nil {
			log.Printf("Error requesting Championship playoffs for %d: %v", year, err)
		}

		// Consolation Bracket
		consURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/playoffs?bracketType=consolation&standingsTab=playoffs", leagueId, year)
		ctxCons := colly.NewContext()
		ctxCons.Put("year", year)
		ctxCons.Put("bracketType", "Consolation")

		err = c.Request("GET", consURL, nil, ctxCons, nil)
		if err != nil {
			log.Printf("Error requesting Consolation playoffs for %d: %v", year, err)
		}
	}

	c.Wait()

	// Sort by Year (ascending) and then by Week (ascending)
	sort.Slice(allGames, func(i, j int) bool {
		if allGames[i].Year != allGames[j].Year {
			return allGames[i].Year < allGames[j].Year
		}
		return allGames[i].Week < allGames[j].Week
	})

	// Write to JSON file
	file, err := os.Create("playoff-history.json")
	if err != nil {
		log.Printf("Error creating playoff-history.json: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allGames); err != nil {
		log.Printf("Error encoding playoff history to JSON: %v\n", err)
	} else {
		fmt.Println("Successfully saved playoff history to playoff-history.json")
	}
}

func getPlayoffRoundNumber(label string) int {
	l := strings.ToLower(label)
	if strings.Contains(l, "quarterfinal") {
		return 1
	}
	if strings.Contains(l, "semifinal") {
		return 2
	}
	if strings.Contains(l, "final") || strings.Contains(l, "bowl") || strings.Contains(l, "place") || strings.Contains(l, "championship") {
		return 3
	}
	return 0
}
