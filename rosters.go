package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

type TeamRoster struct {
	Year    int
	TeamID  string
	Players []RosterPlayer
}

type RosterPlayer struct {
	PlayerID       string
	PlayerName     string
	RosterPosition string // e.g., QB, RB, BN, RES
	Team           string // NFL Team
	TeamPosition   string // e.g., QB, RB
	Points         float32
	IsStarting     bool
}

func scrapeRosters() {
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	rosterCollector := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 4,
	})

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
		classAttr := e.ChildAttr(".teamImg", "class")
		if matches := teamIDRegex.FindStringSubmatch(classAttr); len(matches) > 1 {
			teamID = matches[1]
		}

		var players []RosterPlayer

		// Iterate over all player tables (Offense, Kicker, Defense)
		e.ForEach(".tableWrap table tbody tr", func(_ int, el *colly.HTMLElement) {
			p := parseRosterPlayerRow(el)
			if p.PlayerID != "" {
				players = append(players, p)
				status := "Starter"
				if !p.IsStarting {
					status = "Bench"
				}
				fmt.Printf("    [Roster] Year: %d | Team: %-2s | %-7s | %-22s | %-3s | %-3s | %6.2f\n",
					year, teamID, status, p.PlayerName, p.RosterPosition, p.Team, p.Points)
			}
		})

		if len(players) > 0 {
			fmt.Printf("Completed Roster: Year %d, Team %s, Total Players: %d\n", year, teamID, len(players))
		}
	})

	// Start the process
	for year := startYear; year <= endYear; year++ {
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/owners", leagueId, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		fmt.Printf("Finding teams for roster scraping in %d...\n", year)
		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Println("Error visiting owners page:", err)
		}
	}

	c.Wait()
	rosterCollector.Wait()
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
	rosterPos = mapToSleeperPosition(rosterPos)

	// 4. Team and Position Info
	teamPosText := e.ChildText(".playerNameAndInfo em")
	teamPos := ""
	team := ""
	if matches := playerTeamAndPositionRegex.FindStringSubmatch(teamPosText); len(matches) > 1 {
		teamPos = matches[1]
		team = matches[2]
	} else {
		if strings.Contains(teamPosText, "DEF") {
			team = mapTeamAbbreviation(name)
			teamPos = "DEF"
		} else {
			teamPos = teamPosText
		}
	}

	// 5. Points
	ptsStr := e.ChildText(".statTotal")
	ptsStr = strings.ReplaceAll(ptsStr, "-", "0")
	pts, _ := strconv.ParseFloat(strings.TrimSpace(ptsStr), 32)

	// 6. Check if Starter or Bench
	isStarting := true
	if rosterPos == "BN" || rosterPos == "RES" {
		isStarting = false
	}

	return RosterPlayer{
		PlayerID:       idStr,
		PlayerName:     name,
		RosterPosition: rosterPos,
		Team:           team,
		TeamPosition:   teamPos,
		Points:         float32(pts),
		IsStarting:     isStarting,
	}
}
