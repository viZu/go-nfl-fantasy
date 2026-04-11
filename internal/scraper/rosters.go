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

type TeamRoster struct {
	Year    int            `json:"year"`
	TeamID  string         `json:"teamId"`
	Players []RosterPlayer `json:"players"`
}

type RosterPlayer struct {
	StarterType    string  `json:"starterType"`
	PlayerName     string  `json:"playerName"`
	PlayerID       string  `json:"playerId"`
	RosterPosition string  `json:"rosterPosition"`
	Team           string  `json:"team"`
	TeamPosition   string  `json:"teamPosition"`
	Points         float32 `json:"points"`
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

	var allRosters []TeamRoster
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

		var players []RosterPlayer

		// Iterate over all player tables (Offense, Kicker, Defense)
		e.ForEach(".tableWrap table tbody tr", func(_ int, el *colly.HTMLElement) {
			p := parseRosterPlayerRow(el)
			if p.PlayerID != "" {
				players = append(players, p)
			}
		})

		if len(players) > 0 {
			mu.Lock()
			allRosters = append(allRosters, TeamRoster{
				Year:    year,
				TeamID:  teamID,
				Players: players,
			})
			mu.Unlock()
		}
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
	sort.Slice(allRosters, func(i, j int) bool {
		if allRosters[i].Year != allRosters[j].Year {
			return allRosters[i].Year < allRosters[j].Year
		}
		idI, _ := strconv.Atoi(allRosters[i].TeamID)
		idJ, _ := strconv.Atoi(allRosters[j].TeamID)
		return idI < idJ
	})

	// 4. Write to JSON file
	file, err := os.Create("end-roster-history.json")
	if err != nil {
		log.Printf("❌ [ROSTERS] Error creating end-roster-history.json: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allRosters); err != nil {
		log.Printf("❌ [ROSTERS] Error encoding rosters to JSON: %v\n", err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d rosters to end-roster-history.json (took %s)\n", len(allRosters), time.Since(startTime))
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
	teamPos := ""
	team := ""
	if matches := playerTeamAndPositionRegex.FindStringSubmatch(teamPosText); len(matches) > 1 {
		part1 := matches[1]
		part2 := matches[2]

		if part2 == "DEF" {
			team = utils.MapTeamAbbreviation(part1)
			teamPos = "DEF"
		} else {
			team = part1
			teamPos = part2
		}
	} else {
		if strings.Contains(teamPosText, "DEF") {
			team = utils.MapTeamAbbreviation(name)
			teamPos = "DEF"
		} else {
			teamPos = teamPosText
		}
	}

	// 5. Points
	ptsStr := e.ChildText(".statTotal")
	ptsStr = strings.ReplaceAll(ptsStr, "-", "0")
	pts, _ := strconv.ParseFloat(strings.TrimSpace(ptsStr), 32)

	return RosterPlayer{
		StarterType:    starterType,
		PlayerName:     name,
		PlayerID:       idStr,
		RosterPosition: rosterPosMapped,
		Team:           team,
		TeamPosition:   teamPos,
		Points:         float32(pts),
	}
}
