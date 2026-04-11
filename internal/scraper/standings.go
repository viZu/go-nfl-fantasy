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

type TeamStanding struct {
	TeamID        string  `json:"teamId"`
	TeamName      string  `json:"teamName"`
	DivisionRank  int     `json:"divisionRank"`
	OverallRank   int     `json:"overallRank"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	Draws         int     `json:"draws"`
	PointsFor     float32 `json:"pointsFor"`
	PointsAgainst float32 `json:"pointsAgainst"`
}

type DivisionStanding struct {
	Year         int            `json:"year"`
	DivisionId   int            `json:"divisionId"`
	DivisionName string         `json:"divisionName"`
	Teams        []TeamStanding `json:"teams"`
}

var recordRegex = regexp.MustCompile(`(\d+)-(\d+)-(\d+)`)
var rankRegex = regexp.MustCompile(`(\d+)\s*\((\d+)\)`)
var divisionRegex = regexp.MustCompile(`Division\s+(\d+):\s*(.*)`)

func ScrapeStandings(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[STANDINGS] Starting regular season standings scraper...")

	c := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	var allDivisions []DivisionStanding
	var mu sync.Mutex

	c.OnHTML("#leagueHistoryStandings .bd", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)
		hasDivisions := e.DOM.Find(".tableWrap.hasDivisions").Length() > 0

		e.ForEach(".tableWrap", func(_ int, el *colly.HTMLElement) {
			divRaw := el.ChildText("h4, h5")

			// If we have divisions, the tables with division names are the ones we want.
			// The table without a name is typically the "Overall Standings" which we skip to avoid duplicates.
			if hasDivisions && divRaw == "" {
				return
			}

			divisionId := 1
			divisionName := "Regular Season"

			if divRaw != "" {
				divRaw = strings.TrimSpace(divRaw)
				if matches := divisionRegex.FindStringSubmatch(divRaw); len(matches) > 2 {
					divisionId, _ = strconv.Atoi(matches[1])
					divisionName = matches[2]
				} else {
					divisionName = divRaw
				}
			}

			division := DivisionStanding{
				Year:         year,
				DivisionId:   divisionId,
				DivisionName: divisionName,
				Teams:        make([]TeamStanding, 0),
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
				mu.Lock()
				allDivisions = append(allDivisions, division)
				mu.Unlock()
			}
		})
	})

	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/standings?historyStandingsType=regular", cfg.LeagueID, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("❌ [STANDINGS] Error visiting standings page for %d: %v\n", year, err)
		}
	}

	c.Wait()

	// Sort by Year (ascending) and then by DivisionId (ascending)
	sort.Slice(allDivisions, func(i, j int) bool {
		if allDivisions[i].Year != allDivisions[j].Year {
			return allDivisions[i].Year < allDivisions[j].Year
		}
		return allDivisions[i].DivisionId < allDivisions[j].DivisionId
	})

	// Write to JSON file
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)
	file, err := os.Create(fmt.Sprintf("%s/regular-season-standings-history.json", exportDir))
	if err != nil {
		log.Printf("❌ [STANDINGS] Error creating regular-season-standings-history.json: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allDivisions); err != nil {
		log.Printf("❌ [STANDINGS] Error encoding regular season standings to JSON: %v\n", err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d division records to regular-season-standings-history.json (took %s)\n", len(allDivisions), time.Since(startTime))
	}
}
