package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"

	"github.com/gocolly/colly"
)

// Manager represents the data structure for a single entry
type Manager struct {
	Year        int
	ManagerName string
	UserID      string
	TeamName    string
	TeamID      string
}

type TeamKey struct {
	Year   int
	TeamID string
}

func scrapeManagers() map[TeamKey]Manager {
	c := createColly(nil)

	// Compile regex for extracting numeric IDs from class strings (e.g., "userId-12345")
	userIDRegex := regexp.MustCompile(`userId-(\d+)`)

	// Slice to hold all scraped data
	var allManagers []Manager

	// OnHTML callback for the table rows
	// Based on your file: <table class="tableType-team"> -> <tbody> -> <tr>
	c.OnHTML(".tableType-team tbody tr", func(e *colly.HTMLElement) {

		// 1. Extract Team Name
		// Found in: <a class="teamName">
		teamName := e.ChildText(".teamName")

		// 2. Extract Team ID
		// Found in href: /league/192834/history/2024/teamhome?teamId=7
		href := e.ChildAttr(".teamName", "href")
		teamID := ""
		if href != "" {
			u, err := url.Parse(href)
			if err == nil {
				teamID = u.Query().Get("teamId")
			}
		}

		// 3. Extract Manager Name
		// Found in: <span class="userName">
		managerName := e.ChildText(".teamOwnerName .userName")

		// 4. Extract User ID
		// Found in class attribute: <span class="userName userId-20122">
		classAttr := e.ChildAttr(".teamOwnerName .userName", "class")
		userID := ""
		matches := userIDRegex.FindStringSubmatch(classAttr)
		if len(matches) > 1 {
			userID = matches[1]
		}

		// Get the year from the context (passed during Request)
		year := e.Request.Ctx.GetAny("year").(int)

		// Create struct and append only if we found valid data
		if teamName != "" {
			mgr := Manager{
				Year:        year,
				ManagerName: managerName,
				UserID:      userID,
				TeamName:    teamName,
				TeamID:      teamID,
			}
			allManagers = append(allManagers, mgr)
			fmt.Printf("    [Manager] Year: %d | Team ID: %-2s | User ID: %-6s | %-15s (%s)\n",
				year, teamID, userID, managerName, teamName)
		}
	})

	// Loop through the years and visit URLs
	for year := startYear; year <= endYear; year++ {
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/owners", leagueId, year)

		// Pass the year variable to the context so we can use it in the OnHTML callback
		ctx := colly.NewContext()
		ctx.Put("year", year)

		fmt.Printf("Scraping managers for %d...\n", year)
		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Println("Error visiting page:", err)
		}
	}

	c.Wait()

	return createLookupTable(allManagers)
}

// Function to convert slice to lookup map
func createLookupTable(managers []Manager) map[TeamKey]Manager {
	lookup := make(map[TeamKey]Manager)

	for _, mgr := range managers {
		key := TeamKey{
			Year:   mgr.Year,
			TeamID: mgr.TeamID,
		}
		lookup[key] = mgr
	}

	return lookup
}
