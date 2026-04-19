package scraper

import (
	"encoding/json"
	"fmt"
	"gonflfantasy/internal/config"
	"gonflfantasy/pkg/utils"
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

var rosterTeamIDRegex = regexp.MustCompile(`teamId=(\d+)`)

type RosterPlayer struct {
	Year           int     `json:"year"`
	TeamID         string  `json:"teamId"`
	PlayerID       string  `json:"playerId"`
	StarterType    string  `json:"status"`
	RosterPosition string  `json:"pos"`
	Team           string  `json:"nflTeam"`
	Points         float32 `json:"pts"`
}

func ScrapeRosters(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[ROSTERS] Starting end-of-season rosters scraper...")

	c := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	rosterCollector := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 4,
	})

	var allRosterPlayers []RosterPlayer
	var mu sync.Mutex

	// 1. Visit the Owners page to find all team links
	c.OnHTML(".tableType-team tbody tr", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)
		teamLink := e.ChildAttr(".teamName", "href")

		if teamLink != "" {
			ctx := colly.NewContext()
			ctx.Put("year", year)
			rosterCollector.Request("GET", e.Request.AbsoluteURL(teamLink), nil, ctx, nil)
		}
	})

	// 2. Scrape the Roster page
	rosterCollector.OnHTML("#teamHome", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)

		// Extract Team ID
		teamID := ""
		formAction := e.ChildAttr("form", "action")
		if matches := rosterTeamIDRegex.FindStringSubmatch(formAction); len(matches) > 1 {
			teamID = matches[1]
		}

		// Iterate over all player tables (Offense, Kicker, Defense)
		e.ForEach(".tableWrap table tbody tr", func(_ int, el *colly.HTMLElement) {
			p := parseRosterPlayerRow(el)
			if p.PlayerID != "" {
				p.Year = year
				p.TeamID = teamID

				mu.Lock()
				allRosterPlayers = append(allRosterPlayers, p)
				mu.Unlock()
			}
		})
	})

	// Start the process
	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/owners", cfg.LeagueID, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("❌ [ROSTERS] Error visiting owners page for year %d: %v\n", year, err)
		}
	}

	c.Wait()
	rosterCollector.Wait()

	// 5. Sort by 1. Year and 2. TeamID (numeric)
	sort.Slice(allRosterPlayers, func(i, j int) bool {
		if allRosterPlayers[i].Year != allRosterPlayers[j].Year {
			return allRosterPlayers[i].Year < allRosterPlayers[j].Year
		}
		idI, _ := strconv.Atoi(allRosterPlayers[i].TeamID)
		idJ, _ := strconv.Atoi(allRosterPlayers[j].TeamID)
		return idI < idJ
	})

	// Group rosters by year
	rostersByYear := groupRostersByYear(allRosterPlayers)
	years := getSortedRostersYears(rostersByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writeRostersYear(year, rostersByYear[year], exportDir)
	}
	fmt.Printf("\t✅ Completed end-of-season rosters scraping (took %s)\n", time.Since(startTime))
}

func groupRostersByYear(allRosters []RosterPlayer) map[int][]RosterPlayer {
	rostersByYear := make(map[int][]RosterPlayer)
	for _, roster := range allRosters {
		rostersByYear[roster.Year] = append(rostersByYear[roster.Year], roster)
	}
	return rostersByYear
}

func getSortedRostersYears(rostersByYear map[int][]RosterPlayer) []int {
	var years []int
	for year := range rostersByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writeRostersYear(year int, yearRosters []RosterPlayer, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	fileName := "end-roster-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [ROSTERS] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearRosters); err != nil {
		log.Printf("❌ [ROSTERS] Error encoding rosters to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d rosters to %d/%s\n", len(yearRosters), year, fileName)
	}
}

func parseRosterPlayerRow(e *colly.HTMLElement) RosterPlayer {
	// 1. Player ID
	idStr := ""
	nameClass := e.ChildAttr(".playerNameAndInfo .playerName", "class")
	if matches := playerIDRegex.FindStringSubmatch(nameClass); len(matches) > 1 {
		idStr = matches[1]
	}

	// 2. Full Name
	name := e.ChildText(".playerNameAndInfo .playerName")

	// 3. Roster Position (Starting position on the team)
	rosterPos := e.ChildText(".teamPosition")
	rosterPosMapped, starterType := utils.MapToSleeperPosition(rosterPos)

	// 4. Team and Position Info
	teamPosText := e.ChildText(".playerNameAndInfo em")
	team := ""
	if matches := playerTeamAndPositionRegex.FindStringSubmatch(teamPosText); len(matches) > 1 {
		part1 := matches[1]
		part2 := matches[2]

		if part2 == "DEF" {
			team = utils.MapTeamAbbreviation(part1)
		} else {
			team = part2
		}
	} else {
		if strings.Contains(teamPosText, "DEF") {
			team = utils.MapTeamAbbreviation(name)
		}
	}

	// 5. Points
	ptsStr := e.ChildText(".statTotal")
	ptsStr = strings.ReplaceAll(ptsStr, "-", "0")
	pts, _ := strconv.ParseFloat(strings.TrimSpace(ptsStr), 32)

	return RosterPlayer{
		StarterType:    starterType,
		PlayerID:       idStr,
		RosterPosition: rosterPosMapped,
		Team:           team,
		Points:         float32(pts),
	}
}
