package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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
	stats            map[string]float32
}

// Regex helpers
var playerTeamAndPositionRegex = regexp.MustCompile(`(.*) - (.*)`)
var teamIDRegex = regexp.MustCompile(`teamId-(\d+)`)
var playerIDRegex = regexp.MustCompile(`playerNameId-(\d+)`)
var weekRegex = regexp.MustCompile(`week=(\d+)`) // To extract week from URL if needed

func transformToFBS(originalURL string) (string, error) {
	u, err := url.Parse(originalURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("gameCenterTab", "track")
	q.Set("trackType", "fbs")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func scrapeMatchups() {
	// Storage
	var allMatchups []Matchup
	var mu sync.Mutex

	year := endYear
	scheduleCollector := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
		Delay:       200 * time.Millisecond,
		RandomDelay: 500 * time.Millisecond,
	})

	// Base URLs
	baseURL := "https://fantasy.nfl.com"
	startURL := fmt.Sprintf("%s/league/%s/history/%d/schedule", baseURL, leagueId, year)

	matchupCollector := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
		Delay:       100 * time.Millisecond,
		RandomDelay: 250 * time.Millisecond,
	})

	// A. Find all Weeks
	scheduleCollector.OnHTML("ul.scheduleWeekNav", func(e *colly.HTMLElement) {
		e.ForEach("li.ww:not(.selected) a", func(_ int, el *colly.HTMLElement) {
			link := el.Attr("href")
			e.Request.Visit(link)
		})
	})

	// B. Find Matchups on the current Schedule Page
	scheduleCollector.OnHTML("ul.scheduleContent", func(e *colly.HTMLElement) {
		weekStr := e.DOM.ParentsUntil("div.mod").Find("ul.scheduleWeekNav li.selected .title span").Text()
		week, _ := strconv.Atoi(strings.TrimSpace(weekStr))

		e.ForEach("li.matchup", func(i int, el *colly.HTMLElement) {
			matchupIdx := i + 1
			link := el.ChildAttr(".matchupLink a", "href")
			if link != "" {
				absURL := e.Request.AbsoluteURL(link)
				fbsURL, err := transformToFBS(absURL)
				if err != nil {
					log.Printf("Error transforming URL: %v", err)
					fbsURL = absURL
				}

				ctx := colly.NewContext()
				ctx.Put("matchupId", strconv.Itoa(matchupIdx))
				ctx.Put("year", strconv.Itoa(year))
				ctx.Put("week", strconv.Itoa(week))

				matchupCollector.Request("GET", fbsURL, nil, ctx, nil)
			}
		})
	})

	// --- Handlers: Matchup Page (Game Center - Full Box Score) ---

	matchupCollector.OnHTML("#teamMatchupFull", func(e *colly.HTMLElement) {
		matchupID, _ := strconv.Atoi(e.Request.Ctx.Get("matchupId"))
		currentYear, _ := strconv.Atoi(e.Request.Ctx.Get("year"))
		currentWeek, _ := strconv.Atoi(e.Request.Ctx.Get("week"))
		if currentWeek == 0 {
			if matches := weekRegex.FindStringSubmatch(e.Request.URL.String()); len(matches) > 1 {
				currentWeek, _ = strconv.Atoi(matches[1])
			}
		}

		extractTeamData := func(wrapClass string) Matchup {
			// 1. Get Team ID and Points from the header section
			teamIDStr := ""
			classAttr := e.ChildAttr(wrapClass+" .teamTotal", "class")
			if matches := teamIDRegex.FindStringSubmatch(classAttr); len(matches) > 1 {
				teamIDStr = matches[1]
			}
			teamID, _ := strconv.Atoi(teamIDStr)

			pointsStr := e.ChildText(wrapClass + " .teamTotal")
			points, _ := strconv.ParseFloat(strings.TrimSpace(pointsStr), 32)

			// 2. Extract Players from the Full Box Score section
			var starters []Player
			var bench []Player

			e.ForEach(wrapClass+" .tableWrap table", func(_ int, table *colly.HTMLElement) {
				// Build header map for stats
				statHeaders := buildStatHeaders(table)

				table.ForEach("tbody tr", func(_ int, el *colly.HTMLElement) {
					p := parsePlayerRow(el, statHeaders)
					if p.id != "" {
						// Debug Print for Player Stats
						statsParts := []string{}
						for k, v := range p.stats {
							if v != 0 {
								statsParts = append(statsParts, fmt.Sprintf("%s:%.1f", k, v))
							}
						}
						fmt.Printf("      [Player] %-3s | %-20s | Pts: %6.2f | Stats: %s\n",
							p.startingPosition, p.name, p.points, strings.Join(statsParts, " "))

						if p.startingPosition == "BN" || p.startingPosition == "RES" {
							bench = append(bench, p)
						} else {
							starters = append(starters, p)
						}
					}
				})
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

		m1 := extractTeamData(".teamWrap-1")
		m2 := extractTeamData(".teamWrap-2")

		mu.Lock()
		allMatchups = append(allMatchups, m1, m2)
		mu.Unlock()

		fmt.Printf("Scraped Wk %d Matchup %d: Team %d (%.2f) vs Team %d (%.2f)\n",
			currentWeek, matchupID, m1.team, m1.points, m2.team, m2.points)
	})

	scheduleCollector.OnError(func(r *colly.Response, err error) {
		log.Println("Schedule Collector Error:", err, r.Request.URL)
	})
	matchupCollector.OnError(func(r *colly.Response, err error) {
		log.Println("Matchup Collector Error:", err, r.Request.URL)
	})

	fmt.Println("Starting scraper...")
	scheduleCollector.Visit(startURL)
	scheduleCollector.Wait()
	matchupCollector.Wait()
}

func buildStatHeaders(table *colly.HTMLElement) []string {
	var headers []string

	// Map to store groups by column index
	groupMap := make(map[int]string)

	// First row of header contains groupings
	table.DOM.Find("thead tr.first th").Each(func(i int, s *goquery.Selection) {
		groupName := strings.TrimSpace(s.Find("span").Text())

		// Map specific groups as requested
		lowGroupName := strings.ToLower(groupName)
		if strings.Contains(lowGroupName, "pat") {
			groupName = "pat"
		} else if strings.Contains(lowGroupName, "fg made") {
			groupName = "fg_made"
		} else if strings.Contains(lowGroupName, "turnover") {
			groupName = "turnover"
		} else if strings.Contains(lowGroupName, "score") {
			groupName = "score"
		} else if strings.Contains(lowGroupName, "misc") {
			groupName = ""
		}

		colspanStr, _ := s.Attr("colspan")
		colspan, _ := strconv.Atoi(colspanStr)
		if colspan == 0 {
			colspan = 1
		}

		// We need to track the actual starting index in the final flat list
		startIdx := len(groupMap)
		for j := 0; j < colspan; j++ {
			groupMap[startIdx+j] = strings.ToLower(groupName)
		}
	})

	// Second row of header contains individual stat labels
	table.DOM.Find("thead tr.last th").Each(func(i int, s *goquery.Selection) {
		if s.HasClass("stat") {
			statLabel := strings.ToLower(strings.TrimSpace(s.Find("span").Text()))
			group := groupMap[i]

			fullName := statLabel
			if group != "" && group != "fantasy" && group != "fum" {
				fullName = group + "_" + statLabel
			}
			headers = append(headers, fullName)
		} else {
			// Placeholder for non-stat columns to keep indices aligned
			headers = append(headers, "")
		}
	})

	return headers
}

func parsePlayerRow(e *colly.HTMLElement, statHeaders []string) Player {
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
	startingPosition = mapToSleeperPosition(startingPosition)
	teamPositionText := e.ChildText(".playerNameAndInfo em")
	teamPosition := ""
	team := ""
	if matches := playerTeamAndPositionRegex.FindStringSubmatch(teamPositionText); len(matches) > 1 {
		teamPosition = matches[1]
		team = matches[2]
	} else {
		if strings.Contains(teamPositionText, "DEF") {
			team = mapTeamAbbreviation(name)
			teamPosition = "DEF"
		} else {
			teamPosition = teamPositionText
		}
	}

	// Parse Points
	ptsStr := e.ChildText(".statTotal")
	ptsStr = strings.ReplaceAll(ptsStr, "-", "0")
	pts, _ := strconv.ParseFloat(strings.TrimSpace(ptsStr), 32)

	// Parse Stats
	stats := make(map[string]float32)
	e.DOM.Find("td").Each(func(i int, s *goquery.Selection) {
		if i < len(statHeaders) && statHeaders[i] != "" {
			valStr := strings.TrimSpace(s.Text())
			valStr = strings.ReplaceAll(valStr, "-", "0")
			valStr = strings.ReplaceAll(valStr, ",", "")
			val, _ := strconv.ParseFloat(valStr, 32)
			stats[statHeaders[i]] = float32(val)
		}
	})

	return Player{
		id:               idStr,
		name:             name,
		startingPosition: startingPosition,
		points:           float32(pts),
		teamPosition:     teamPosition,
		team:             team,
		stats:            stats,
	}
}
