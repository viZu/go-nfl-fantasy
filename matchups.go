package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

type Matchup struct {
	matchupId int
	year      int
	week      int
	team      int
	points    float32
	starters  []Player
	bench     []Player
}

type Player struct {
	id               string
	name             string
	points           float32
	teamPosition     string
	team             string
	startingPosition string
}

// Regex helpers
var playerTeamAndPositionRegex = regexp.MustCompile(`(.*) - (.*)`)
var teamIDRegex = regexp.MustCompile(`teamId-(\d+)`)
var playerIDRegex = regexp.MustCompile(`playerNameId-(\d+)`)
var weekRegex = regexp.MustCompile(`week=(\d+)`) // To extract week from URL if needed

func scrapeMatchups() {
	// Storage
	var allMatchups []Matchup

	year := endYear
	scheduleCollector := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 4,
		Delay:       200 * time.Millisecond,
		RandomDelay: 500 * time.Millisecond,
	})

	// Base URLs
	baseURL := "https://fantasy.nfl.com"
	startURL := fmt.Sprintf("%s/league/%s/history/%d/schedule", baseURL, leagueId, year)

	matchupCollector := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 8,
		Delay:       100 * time.Millisecond,
		RandomDelay: 250 * time.Millisecond,
	})

	// A. Find all Weeks
	// The schedule page usually defaults to the current week.
	// We grab links to all other weeks from the nav bar to ensure we scrape the full season.
	scheduleCollector.OnHTML("ul.scheduleWeekNav", func(e *colly.HTMLElement) {
		e.ForEach("li.ww:not(.selected) a", func(_ int, el *colly.HTMLElement) {
			link := el.Attr("href")
			// These links are usually relative like "schedule?gameSeason=..."
			// Visit them using the main collector to find matchups on those weeks
			e.Request.Visit(link)
		})
	})

	// B. Find Matchups on the current Schedule Page
	scheduleCollector.OnHTML("ul.scheduleContent", func(e *colly.HTMLElement) {
		// Extract the current week number from the selected nav item
		weekStr := e.DOM.ParentsUntil("div.mod").Find("ul.scheduleWeekNav li.selected .title span").Text()
		week, _ := strconv.Atoi(strings.TrimSpace(weekStr))

		// Iterate over each matchup in the list
		e.ForEach("li.matchup", func(i int, el *colly.HTMLElement) {
			// Matchup ID is the index + 1
			matchupIdx := i + 1

			// Find the Game Center link
			link := el.ChildAttr(".matchupLink a", "href")
			if link != "" {
				// Pass metadata to the Matchup Collector
				ctx := colly.NewContext()
				ctx.Put("matchupId", strconv.Itoa(matchupIdx))
				ctx.Put("year", strconv.Itoa(year))
				ctx.Put("week", strconv.Itoa(week))

				matchupCollector.Request("GET", e.Request.AbsoluteURL(link), nil, ctx, nil)
			}
		})
	})

	// --- Handlers: Matchup Page (Game Center) ---

	matchupCollector.OnHTML("#teamMatchupBoxScore", func(e *colly.HTMLElement) {
		// Retrieve context data
		matchupID, _ := strconv.Atoi(e.Request.Ctx.Get("matchupId"))
		currentYear, _ := strconv.Atoi(e.Request.Ctx.Get("year"))

		// Attempt to get week from context, fallback to URL parsing if context is missing (edge case)
		currentWeek, _ := strconv.Atoi(e.Request.Ctx.Get("week"))
		if currentWeek == 0 {
			if matches := weekRegex.FindStringSubmatch(e.Request.URL.String()); len(matches) > 1 {
				currentWeek, _ = strconv.Atoi(matches[1])
			}
		}

		// Helper to extract a single team's data from the page
		extractTeamData := func(wrapClass string, benchID string) Matchup {
			// 1. Get Team ID
			teamIDStr := ""
			// Looks for class="teamName teamId-5"
			classAttr := e.ChildAttr(wrapClass+" .teamTotal", "class")
			if matches := teamIDRegex.FindStringSubmatch(classAttr); len(matches) > 1 {
				teamIDStr = matches[1]
			}
			teamID, _ := strconv.Atoi(teamIDStr)

			// 2. Get Team Points
			pointsStr := e.ChildText(wrapClass + " .teamTotal") // "150.88"
			points, _ := strconv.ParseFloat(strings.TrimSpace(pointsStr), 32)

			// 3. Extract Starters (First table in the wrapper)
			var starters []Player
			e.ForEach(wrapClass+" .tableWrap:not(.tableWrapBN) table tbody tr", func(_ int, el *colly.HTMLElement) {
				p := parsePlayerRow(el)
				if p.id != "" {
					starters = append(starters, p)
				}
			})

			// 4. Extract Bench (Specific ID for bench table)
			var bench []Player
			e.ForEach("#"+benchID+" table tbody tr", func(_ int, el *colly.HTMLElement) {
				p := parsePlayerRow(el)
				if p.id != "" {
					bench = append(bench, p)
				}
			})

			return Matchup{
				matchupId: matchupID,
				year:      currentYear,
				week:      currentWeek,
				team:      teamID,
				points:    float32(points),
				starters:  starters,
				bench:     bench,
			}
		}

		// Extract Team 1
		m1 := extractTeamData(".teamWrap-1", "tableWrapBN-1")
		// Extract Team 2
		m2 := extractTeamData(".teamWrap-2", "tableWrapBN-2")

		// Append to global list (thread-safe append logic might be needed in prod, usually handled by colly sync or channel)
		allMatchups = append(allMatchups, m1, m2)

		fmt.Printf("Scraped Wk %d Matchup %d: Team %d (%.2f) vs Team %d (%.2f)\n",
			currentWeek, matchupID, m1.team, m1.points, m2.team, m2.points)
	})

	// Error Handling
	scheduleCollector.OnError(func(r *colly.Response, err error) {
		log.Println("Schedule Collector Error:", err, r.Request.URL)
	})
	matchupCollector.OnError(func(r *colly.Response, err error) {
		log.Println("Matchup Collector Error:", err, r.Request.URL)
	})

	// Start Scraping
	fmt.Println("Starting scraper...")
	scheduleCollector.Visit(startURL)
	scheduleCollector.Wait()
	matchupCollector.Wait()
}

func parsePlayerRow(e *colly.HTMLElement) Player {
	// Parse ID
	idStr := ""
	nameClass := e.ChildAttr(".playerNameAndInfo .playerName", "class")
	if matches := playerIDRegex.FindStringSubmatch(nameClass); len(matches) > 1 {
		idStr = matches[1]
	}

	// Parse Name
	name := e.ChildText(".playerNameAndInfo .playerName")

	// Parse Position
	startingPosition := e.ChildText(".teamPosition")
	teamPositionText := e.ChildText(".playerNameAndInfo em")
	teamPosition := ""
	team := ""
	if matches := playerTeamAndPositionRegex.FindStringSubmatch(teamPositionText); len(matches) > 1 {
		teamPosition = matches[1]
		team = matches[2]
	} else {
		teamPosition = teamPositionText
	}

	// Parse Points
	ptsStr := e.ChildText(".statTotal")
	// Handle cases like "-" or empty
	ptsStr = strings.ReplaceAll(ptsStr, "-", "0")
	pts, _ := strconv.ParseFloat(strings.TrimSpace(ptsStr), 32)

	return Player{
		id:               idStr,
		name:             name,
		startingPosition: startingPosition,
		points:           float32(pts),
		teamPosition:     teamPosition,
		team:             team,
	}
}
