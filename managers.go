package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

// Manager represents the data structure for a single entry
type Manager struct {
	Year            int     `json:"year"`
	ManagerName     string  `json:"managerName"`
	UserID          string  `json:"userId"`
	CoManagerName   *string `json:"coManagerName"`
	CoManagerUserID *string `json:"coUserId"`
	TeamName        string  `json:"teamName"`
	TeamID          string  `json:"teamId"`
	TeamImageURL    string  `json:"teamImgUrl"`
}

func scrapeManagers() {
	startTime := time.Now()
	fmt.Println("[MANAGERS] Starting managers history scraper...")
	c := createColly(nil)

	// Compile regex for extracting numeric IDs from class strings (e.g., "userId-12345")
	userIDRegex := regexp.MustCompile(`userId-(\d+)`)

	// Slice to hold all scraped data
	var allManagers []Manager
	var mu sync.Mutex

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

		// 5. Extract Co-Manager Name and User ID (if present)
		coManagerNameRaw := e.ChildText(".teamCoManagerName .userName")
		var coManagerName *string
		if coManagerNameRaw != "" {
			coManagerName = &coManagerNameRaw
		}

		coManagerUserIDRaw := ""
		coClassAttr := e.ChildAttr(".teamCoManagerName .userName", "class")
		if coClassAttr != "" {
			coMatches := userIDRegex.FindStringSubmatch(coClassAttr)
			if len(coMatches) > 1 {
				coManagerUserIDRaw = coMatches[1]
			}
		}
		var coManagerUserID *string
		if coManagerUserIDRaw != "" {
			coManagerUserID = &coManagerUserIDRaw
		}

		// 6. Extract Team Image URL
		teamImageURL := e.ChildAttr(".teamImg img", "src")

		// Get the year from the context (passed during Request)
		year := e.Request.Ctx.GetAny("year").(int)

		// Create struct and append only if we found valid data
		if teamName != "" {
			mgr := Manager{
				Year:            year,
				ManagerName:     managerName,
				UserID:          userID,
				CoManagerName:   coManagerName,
				CoManagerUserID: coManagerUserID,
				TeamName:        teamName,
				TeamID:          teamID,
				TeamImageURL:    teamImageURL,
			}

			mu.Lock()
			allManagers = append(allManagers, mgr)
			mu.Unlock()
		}
	})

	// Loop through the years and visit URLs
	for year := startYear; year <= endYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/owners", leagueId, year)

		// Pass the year variable to the context so we can use it in the OnHTML callback
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("❌ [MANAGERS] Error visiting page for year %d: %v\n", year, err)
		}
	}

	c.Wait()

	// Sort by Year (ascending) and then by TeamID (numerically ascending)
	sort.Slice(allManagers, func(i, j int) bool {
		if allManagers[i].Year != allManagers[j].Year {
			return allManagers[i].Year < allManagers[j].Year
		}
		idI, _ := strconv.Atoi(allManagers[i].TeamID)
		idJ, _ := strconv.Atoi(allManagers[j].TeamID)
		return idI < idJ
	})

	// Write to JSON file
	file, err := os.Create("managers-history.json")
	if err != nil {
		log.Printf("❌ [MANAGERS] Error creating managers-history.json: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allManagers); err != nil {
		log.Printf("❌ [MANAGERS] Error encoding manager history to JSON: %v\n", err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d managers to managers-history.json (took %s)\n", len(allManagers), time.Since(startTime))
	}
}
