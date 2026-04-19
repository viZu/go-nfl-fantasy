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
	"sync"
	"time"

	"github.com/gocolly/colly"
)

type EndStanding struct {
	Year     int    `json:"year"`
	Rank     int    `json:"rank"`
	TeamID   string `json:"teamId"`
	TeamName string `json:"teamName"`
}

var placeRegex = regexp.MustCompile(`place-(\d+)`)

func ScrapeEndStandings(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[END STANDINGS] Starting end standings scraper...")
	c := CreateColly(cfg, &colly.LimitRule{
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

	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/standings", cfg.LeagueID, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("❌ [END STANDINGS] Error requesting end standings for %d: %v", year, err)
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

	// Group end standings by year
	endStandingsByYear := groupEndStandingsByYear(allStandings)
	years := getSortedEndStandingsYears(endStandingsByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writeEndStandingsYear(year, endStandingsByYear[year], exportDir)
	}
	fmt.Printf("\t✅ Completed end standings history scraping (took %s)\n", time.Since(startTime))
}

func groupEndStandingsByYear(allStandings []EndStanding) map[int][]EndStanding {
	standingsByYear := make(map[int][]EndStanding)
	for _, standing := range allStandings {
		standingsByYear[standing.Year] = append(standingsByYear[standing.Year], standing)
	}
	return standingsByYear
}

func getSortedEndStandingsYears(standingsByYear map[int][]EndStanding) []int {
	var years []int
	for year := range standingsByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writeEndStandingsYear(year int, yearStandings []EndStanding, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	fileName := "end-standings-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [END STANDINGS] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearStandings); err != nil {
		log.Printf("❌ [END STANDINGS] Error encoding end standings to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d records to %d/%s\n", len(yearStandings), year, fileName)
	}
}
