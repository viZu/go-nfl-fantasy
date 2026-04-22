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

type PlayoffGame struct {
	Year        int     `json:"year"`
	Week        int     `json:"week"`
	Round       int     `json:"round"`
	RoundLabel  string  `json:"roundLabel"`
	BracketType string  `json:"bracketType"`
	Team1       string  `json:"team1Id"`
	Team1Seed   int     `json:"team1Seed"`
	Team2       string  `json:"team2Id"`
	Team2Seed   int     `json:"team2Seed"`
	Team1Points float32 `json:"team1Points"`
	Team2Points float32 `json:"team2Points"`
	Winner      string  `json:"winner"`
}

// Local regex for playoff week labels
var playoffWeekRegex = regexp.MustCompile(`Week (\d+)`)
var playoffWeekIndexRegex = regexp.MustCompile(`pw-(\d+)`)
var playoffSeedRegex = regexp.MustCompile(`\((\d+)\)`)

func ScrapePlayoffs(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[PLAYOFFS] Starting playoffs history scraper...")

	c := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})
	c.Async = false

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

		weekIndexStr := e.Attr("class")
		roundNumber := 0
		if matches := playoffWeekIndexRegex.FindStringSubmatch(weekIndexStr); len(matches) > 1 {
			roundNumber, _ = strconv.Atoi(matches[1])
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

			t1SeedStr := el.ChildText(".teamWrap-1 .teamRank")
			t1Seed := 0
			if matches := playoffSeedRegex.FindStringSubmatch(t1SeedStr); len(matches) > 1 {
				t1Seed, _ = strconv.Atoi(matches[1])
			}

			// Team 2 Extraction
			team2ID := ""
			t2Class := el.ChildAttr(".teamWrap-2 .teamName", "class")
			if matches := teamIDRegex.FindStringSubmatch(t2Class); len(matches) > 1 {
				team2ID = matches[1]
			}
			t2PointsStr := el.ChildText(".teamWrap-2 .teamTotal")
			t2Points, _ := strconv.ParseFloat(strings.TrimSpace(t2PointsStr), 32)

			t2SeedStr := el.ChildText(".teamWrap-2 .teamRank")
			t2Seed := 0
			if matches := playoffSeedRegex.FindStringSubmatch(t2SeedStr); len(matches) > 1 {
				t2Seed, _ = strconv.Atoi(matches[1])
			}

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
					Round:       roundNumber + 1,
					RoundLabel:  roundLabel,
					BracketType: bracketType,
					Team1:       team1ID,
					Team1Seed:   t1Seed,
					Team2:       team2ID,
					Team2Seed:   t2Seed,
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

	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		// Championship Bracket
		champURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/playoffs?bracketType=championship&standingsTab=playoffs", cfg.LeagueID, year)
		ctxChamp := colly.NewContext()
		ctxChamp.Put("year", year)
		ctxChamp.Put("bracketType", "Championship")

		err := c.Request("GET", champURL, nil, ctxChamp, nil)
		if err != nil {
			log.Printf("❌ [PLAYOFFS] Error requesting Championship playoffs for %d: %v", year, err)
		}

		// Consolation Bracket
		consURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/playoffs?bracketType=consolation&standingsTab=playoffs", cfg.LeagueID, year)
		ctxCons := colly.NewContext()
		ctxCons.Put("year", year)
		ctxCons.Put("bracketType", "Consolation")

		err = c.Request("GET", consURL, nil, ctxCons, nil)
		if err != nil {
			log.Printf("❌ [PLAYOFFS] Error requesting Consolation playoffs for %d: %v", year, err)
		}
	}

	c.Wait()

	// Sort by Year (ascending), BracketType (ascending), and then by Week (ascending)
	sort.Slice(allGames, func(i, j int) bool {
		if allGames[i].Year != allGames[j].Year {
			return allGames[i].Year < allGames[j].Year
		}
		if allGames[i].BracketType != allGames[j].BracketType {
			return allGames[i].BracketType < allGames[j].BracketType
		}
		return allGames[i].Week < allGames[j].Week
	})

	// Group playoff games by year
	playoffsByYear := groupPlayoffsByYear(allGames)
	years := getSortedPlayoffsYears(playoffsByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writePlayoffsYear(year, playoffsByYear[year], exportDir)
	}
	fmt.Printf("\t✅ Completed playoff history scraping (took %s)\n", time.Since(startTime))
}

func groupPlayoffsByYear(allGames []PlayoffGame) map[int][]PlayoffGame {
	playoffsByYear := make(map[int][]PlayoffGame)
	for _, game := range allGames {
		playoffsByYear[game.Year] = append(playoffsByYear[game.Year], game)
	}
	return playoffsByYear
}

func getSortedPlayoffsYears(playoffsByYear map[int][]PlayoffGame) []int {
	var years []int
	for year := range playoffsByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writePlayoffsYear(year int, yearGames []PlayoffGame, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	fileName := "playoff-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [PLAYOFFS] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearGames); err != nil {
		log.Printf("❌ [PLAYOFFS] Error encoding playoff history to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d games to %d/%s\n", len(yearGames), year, fileName)
	}
}
