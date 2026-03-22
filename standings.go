package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

type TeamStanding struct {
	TeamID        string
	TeamName      string
	DivisionRank  int
	OverallRank   int
	Wins          int
	Losses        int
	Draws         int
	PointsFor     float32
	PointsAgainst float32
}

type DivisionStanding struct {
	Year         int
	DivisionName string
	Teams        []TeamStanding
}

var recordRegex = regexp.MustCompile(`(\d+)-(\d+)-(\d+)`)
var rankRegex = regexp.MustCompile(`(\d+)\s*\((\d+)\)`)

func scrapeRegularSeasonStandings() {
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	c.OnHTML("#leagueHistoryStandings .bd", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)
		hasDivisions := e.DOM.Find(".tableWrap.hasDivisions").Length() > 0

		var allDivisions []DivisionStanding

		e.ForEach(".tableWrap", func(_ int, el *colly.HTMLElement) {
			divName := el.ChildText("h4, h5")

			// If we have divisions, the tables with division names are the ones we want.
			// The table without a name is typically the "Overall Standings" which we skip to avoid duplicates.
			if hasDivisions && divName == "" {
				return
			}

			if divName == "" {
				divName = "Regular Season"
			} else {
				// Clean up division name (e.g., "Division 1: Dirty South" -> "Dirty South" or keep it)
				// The prompt says: "The division name is within .tableWrap.first h4"
				// Example: <h5 class="first"><em>Division 1</em>: Dirty South</h5>
				// ChildText will return "Division 1: Dirty South"
				divName = strings.TrimSpace(divName)
			}

			division := DivisionStanding{
				Year:         year,
				DivisionName: divName,
			}

			el.ForEach("table tbody tr", func(_ int, tr *colly.HTMLElement) {
				teamID := ""
				teamIDClass := tr.ChildAttr(".teamName", "class")
				if matches := teamIDRegex.FindStringSubmatch(teamIDClass); len(matches) > 1 {
					teamID = matches[1]
				}

				teamName := tr.ChildText(".teamName")

				// Ranks
				rankText := tr.ChildText(".teamRank")
				divRank := 0
				overallRank := 0
				if matches := rankRegex.FindStringSubmatch(rankText); len(matches) > 2 {
					divRank, _ = strconv.Atoi(matches[1])
					overallRank, _ = strconv.Atoi(matches[2])
				}

				// Record
				recordText := tr.ChildText(".teamRecord")
				wins, losses, draws := 0, 0, 0
				if matches := recordRegex.FindStringSubmatch(recordText); len(matches) > 3 {
					wins, _ = strconv.Atoi(matches[1])
					losses, _ = strconv.Atoi(matches[2])
					draws, _ = strconv.Atoi(matches[3])
				}

				// Points
				var points []float32
				tr.ForEach(".teamPts", func(_ int, pt *colly.HTMLElement) {
					pStr := strings.ReplaceAll(pt.Text, ",", "")
					p, _ := strconv.ParseFloat(strings.TrimSpace(pStr), 32)
					points = append(points, float32(p))
				})

				ptsFor := float32(0)
				ptsAgainst := float32(0)
				if len(points) >= 2 {
					ptsFor = points[0]
					ptsAgainst = points[1]
				}

				if teamID != "" {
					standing := TeamStanding{
						TeamID:        teamID,
						TeamName:      teamName,
						DivisionRank:  divRank,
						OverallRank:   overallRank,
						Wins:          wins,
						Losses:        losses,
						Draws:         draws,
						PointsFor:     ptsFor,
						PointsAgainst: ptsAgainst,
					}
					division.Teams = append(division.Teams, standing)
				}
			})

			if len(division.Teams) > 0 {
				allDivisions = append(allDivisions, division)
				fmt.Printf("    [Standings] Year: %d | Division: %s | Teams: %d\n", year, division.DivisionName, len(division.Teams))
				for _, t := range division.Teams {
					fmt.Printf("        %-2s | %-25s | %d-%d-%d | PF: %8.2f | PA: %8.2f\n",
						t.TeamID, t.TeamName, t.Wins, t.Losses, t.Draws, t.PointsFor, t.PointsAgainst)
				}
			}
		})
	})

	for year := startYear; year <= endYear; year++ {
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/standings?historyStandingsType=regular", leagueId, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		fmt.Printf("Scraping regular season standings for %d...\n", year)
		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Println("Error visiting standings page:", err)
		}
	}

	c.Wait()
}
