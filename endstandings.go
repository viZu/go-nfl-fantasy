package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/gocolly/colly"
)

type EndStanding struct {
	Year     int    `json:"year"`
	Rank     int    `json:"rank"`
	TeamID   string `json:"teamId"`
	TeamName string `json:"teamName"`
}

var placeRegex = regexp.MustCompile(`place-(\d+)`)

func scrapeEndStandings() {
	fmt.Println("Scraping end standings...")
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	allStandings := make([]EndStanding, 0)
	var mu sync.Mutex

	c.OnHTML("#championResults .results ul li", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)

		// Extract Rank from class
		rank := 0
		classAttr := e.Attr("class")
		if matches := placeRegex.FindStringSubmatch(classAttr); len(matches) > 1 {
			rank, _ = strconv.Atoi(matches[1])
		}

		// Extract Team ID
		teamID := ""
		teamName := e.ChildText(".teamName")
		teamClass := e.ChildAttr(".teamName", "class")
		if matches := teamIDRegex.FindStringSubmatch(teamClass); len(matches) > 1 {
			teamID = matches[1]
		}

		if teamID != "" {
			standing := EndStanding{
				Year:     year,
				Rank:     rank,
				TeamID:   teamID,
				TeamName: teamName,
			}

			mu.Lock()
			allStandings = append(allStandings, standing)
			mu.Unlock()
		}
	})

	for year := startYear; year <= endYear; year++ {
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/standings", leagueId, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("Error requesting end standings for %d: %v", year, err)
		}
	}

	c.Wait()

	// Sort by Year (ascending) and then by Rank (ascending)
	sort.Slice(allStandings, func(i, j int) bool {
		if allStandings[i].Year != allStandings[j].Year {
			return allStandings[i].Year < allStandings[j].Year
		}
		return allStandings[i].Rank < allStandings[j].Rank
	})

	// Write to JSON file
	file, err := os.Create("end-standings-history.json")
	if err != nil {
		log.Printf("Error creating end-standings-history.json: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allStandings); err != nil {
		log.Printf("Error encoding end standings to JSON: %v\n", err)
	} else {
		fmt.Println("Successfully saved end standings to end-standings-history.json")
	}
}
