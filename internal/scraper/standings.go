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
	Year          int     `json:"year"`
	DivisionId    int     `json:"divisionId"`
	DivisionName  string  `json:"divisionName"`
	TeamID        string  `json:"teamId"`
	DivisionRank  int     `json:"divisionRank"`
	OverallRank   int     `json:"overallRank"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	Draws         int     `json:"draws"`
	PointsFor     float32 `json:"pointsFor"`
	PointsAgainst float32 `json:"pointsAgainst"`
}

var recordRegex = regexp.MustCompile(`(\d+)-(\d+)-(\d+)`)
var divisionRegex = regexp.MustCompile(`Division\s+(\d+):\s*(.*)`)

func ScrapeStandings(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[STANDINGS] Starting regular season standings scraper...")

	c := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	var allTeamStandings []TeamStanding
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

			el.ForEach("table tbody tr", func(_ int, tr *colly.HTMLElement) {
				teamID := ""
				teamIDClass := tr.ChildAttr(".teamName", "class")
				if matches := teamIDRegex.FindStringSubmatch(teamIDClass); len(matches) > 1 {
					teamID = matches[1]
				}

				// Ranks
				rankNodes := tr.DOM.Find(".teamRank")
				divRank := 0
				overallRank := 0

				if rankNodes.Length() > 0 {
					if rankNodes.Length() >= 2 {
						divRankText := strings.TrimSpace(rankNodes.Eq(1).Text())
						if val, err := strconv.Atoi(divRankText); err == nil {
							divRank = val
							overallRank = val
						}
					}
					if rankNodes.Length() >= 3 {
						overallRankText := strings.Trim(rankNodes.Eq(2).Text(), "()")
						if val, err := strconv.Atoi(overallRankText); err == nil {
							overallRank = val
						}
					}
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
						Year:          year,
						DivisionId:    divisionId,
						DivisionName:  divisionName,
						TeamID:        teamID,
						DivisionRank:  divRank,
						OverallRank:   overallRank,
						Wins:          wins,
						Losses:        losses,
						Draws:         draws,
						PointsFor:     ptsFor,
						PointsAgainst: ptsAgainst,
					}
					mu.Lock()
					allTeamStandings = append(allTeamStandings, standing)
					mu.Unlock()
				}
			})
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

	// Sort by Year (ascending), DivisionId (ascending), and OverallRank (ascending)
	sort.Slice(allTeamStandings, func(i, j int) bool {
		if allTeamStandings[i].Year != allTeamStandings[j].Year {
			return allTeamStandings[i].Year < allTeamStandings[j].Year
		}
		if allTeamStandings[i].DivisionId != allTeamStandings[j].DivisionId {
			return allTeamStandings[i].DivisionId < allTeamStandings[j].DivisionId
		}
		return allTeamStandings[i].OverallRank < allTeamStandings[j].OverallRank
	})

	// Group standings by year
	standingsByYear := groupStandingsByYear(allTeamStandings)
	years := getSortedStandingsYears(standingsByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writeStandingsYear(year, standingsByYear[year], exportDir)
	}
	fmt.Printf("\t✅ Completed regular season standings scraping (took %s)\n", time.Since(startTime))
}

func groupStandingsByYear(allStandings []TeamStanding) map[int][]TeamStanding {
	standingsByYear := make(map[int][]TeamStanding)
	for _, standing := range allStandings {
		standingsByYear[standing.Year] = append(standingsByYear[standing.Year], standing)
	}
	return standingsByYear
}

func getSortedStandingsYears(standingsByYear map[int][]TeamStanding) []int {
	var years []int
	for year := range standingsByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writeStandingsYear(year int, yearStandings []TeamStanding, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	fileName := "regular-season-standings-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [STANDINGS] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearStandings); err != nil {
		log.Printf("❌ [STANDINGS] Error encoding regular season standings to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d division records to %d/%s\n", len(yearStandings), year, fileName)
	}
}
