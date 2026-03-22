package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/gocolly/colly"
)

type EndStanding struct {
	Year     int
	Rank     int
	TeamID   string
	TeamName string
}

var placeRegex = regexp.MustCompile(`place-(\d+)`)

func scrapeEndStandings() {
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

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
			fmt.Printf("    [End Standing] Year: %d | Rank: %-2d | Team ID: %-2s | Team: %s\n",
				year, rank, teamID, teamName)
		}
	})

	for year := startYear; year <= endYear; year++ {
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/standings", leagueId, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		fmt.Printf("Scraping end of season standings for %d...\n", year)
		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("Error requesting end standings for %d: %v", year, err)
		}
	}

	c.Wait()
}
