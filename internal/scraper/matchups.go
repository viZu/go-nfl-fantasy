package scraper

import (
	"encoding/json"
	"fmt"
	"gonflfantasy/internal/config"
	"gonflfantasy/pkg/utils"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
)

// --- Exported JSON Structs ---

type MatchupHistory struct {
	Year        int     `json:"year"`
	Week        int     `json:"week"`
	MatchupID   string  `json:"matchupId"`
	Team1ID     string  `json:"team1Id"`
	Team2ID     string  `json:"team2Id"`
	Team1Points float32 `json:"team1Points"`
	Team2Points float32 `json:"team2Points"`
}

type PlayerMatchupStatistic struct {
	Year      int                `json:"-"`
	MatchupID string             `json:"matchup"`
	TeamID    string             `json:"team"`
	PlayerID  string             `json:"playerId"`
	Position  string             `json:"pos"`
	NFLTeam   string             `json:"nflTeam"`
	Status    string             `json:"status"`
	Points    float32            `json:"pts"`
	Stats     map[string]float32 `json:"stats"`
}

type UniquePlayer struct {
	PlayerID   string `json:"playerId"`
	PlayerName string `json:"playerName"`
	Position   string `json:"position"`
}

// --- Internal Parsing Structs ---

type TeamMatchup struct {
	TeamID      string
	TotalPoints float32
	Players     []MatchupPlayer
}

type MatchupPlayer struct {
	PlayerID         string
	PlayerName       string
	Status           string // "ST", "BN", "RES"
	StartingPosition string
	Team             string
	TeamPosition     string
	Points           float32
	Stats            map[string]float32
}

// Regex helpers
var playerTeamAndPositionRegex = regexp.MustCompile(`(.*) - (.*)`)
var teamIDRegex = regexp.MustCompile(`teamId-(\d+)`)
var playerIDRegex = regexp.MustCompile(`playerNameId-(\d+)`)
var weekRegex = regexp.MustCompile(`week=(\d+)`)

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

func ScrapeMatchups(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[MATCHUPS] Starting matchups history scraper...")

	var allMatchups []MatchupHistory
	var allPlayerStats []PlayerMatchupStatistic
	uniquePlayersMap := make(map[string]UniquePlayer)
	var mu sync.Mutex

	scheduleCollector := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
		Delay:       200 * time.Millisecond,
		RandomDelay: 500 * time.Millisecond,
	})

	matchupCollector := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 4,
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
		year := e.Request.Ctx.GetAny("year").(int)
		weekStr := e.DOM.ParentsUntil("div.mod").Find("ul.scheduleWeekNav li.selected .title span").Text()
		week, _ := strconv.Atoi(strings.TrimSpace(weekStr))

		e.ForEach("li.matchup", func(i int, el *colly.HTMLElement) {
			link := el.ChildAttr(".matchupLink a", "href")
			if link != "" {
				absURL := e.Request.AbsoluteURL(link)
				fbsURL, err := transformToFBS(absURL)
				if err != nil {
					log.Printf("❌ [MATCHUPS] Error transforming URL: %v", err)
					fbsURL = absURL
				}

				ctx := colly.NewContext()
				ctx.Put("year", year)
				ctx.Put("week", week)

				matchupCollector.Request("GET", fbsURL, nil, ctx, nil)
			}
		})
	})

	// --- Handlers: Matchup Page (Game Center - Full Box Score) ---

	matchupCollector.OnHTML("#teamMatchupFull", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)
		week := e.Request.Ctx.GetAny("week").(int)
		if week == 0 {
			if matches := weekRegex.FindStringSubmatch(e.Request.URL.String()); len(matches) > 1 {
				week, _ = strconv.Atoi(matches[1])
			}
		}

		extractTeamMatchup := func(wrapClass string) TeamMatchup {
			teamID := ""
			classAttr := e.ChildAttr(wrapClass+" .teamTotal", "class")
			if matches := teamIDRegex.FindStringSubmatch(classAttr); len(matches) > 1 {
				teamID = matches[1]
			}

			pointsStr := e.ChildText(wrapClass + " .teamTotal")
			points, _ := strconv.ParseFloat(strings.TrimSpace(pointsStr), 32)

			var players []MatchupPlayer

			e.ForEach(wrapClass+" .tableWrap table", func(_ int, table *colly.HTMLElement) {
				statHeaders := buildStatHeaders(table)

				table.ForEach("tbody tr", func(_ int, el *colly.HTMLElement) {
					p := parseMatchupPlayerRow(el, statHeaders)
					if p.PlayerID != "" {
						players = append(players, p)
					}
				})
			})

			return TeamMatchup{
				TeamID:      teamID,
				TotalPoints: float32(points),
				Players:     players,
			}
		}

		t1 := extractTeamMatchup(".teamWrap-1")
		t2 := extractTeamMatchup(".teamWrap-2")

		// Ensure team1 always has the lower ID
		id1, _ := strconv.Atoi(t1.TeamID)
		id2, _ := strconv.Atoi(t2.TeamID)
		if id1 > id2 {
			t1, t2 = t2, t1
		}

		matchupID := fmt.Sprintf("%d-%d-%s-%s", year, week, t1.TeamID, t2.TeamID)

		mu.Lock()
		allMatchups = append(allMatchups, MatchupHistory{
			Year:        year,
			Week:        week,
			MatchupID:   matchupID,
			Team1ID:     t1.TeamID,
			Team2ID:     t2.TeamID,
			Team1Points: t1.TotalPoints,
			Team2Points: t2.TotalPoints,
		})

		for _, p := range t1.Players {
			allPlayerStats = append(allPlayerStats, PlayerMatchupStatistic{
				Year:      year,
				MatchupID: matchupID,
				TeamID:    t1.TeamID,
				PlayerID:  p.PlayerID,
				Position:  p.StartingPosition,
				NFLTeam:   p.Team,
				Status:    p.Status,
				Points:    p.Points,
				Stats:     p.Stats,
			})
			uniquePlayersMap[p.PlayerID] = UniquePlayer{
				PlayerID:   p.PlayerID,
				PlayerName: p.PlayerName,
				Position:   p.TeamPosition,
			}
		}
		for _, p := range t2.Players {
			allPlayerStats = append(allPlayerStats, PlayerMatchupStatistic{
				Year:      year,
				MatchupID: matchupID,
				TeamID:    t2.TeamID,
				PlayerID:  p.PlayerID,
				Position:  p.StartingPosition,
				NFLTeam:   p.Team,
				Status:    p.Status,
				Points:    p.Points,
				Stats:     p.Stats,
			})
			uniquePlayersMap[p.PlayerID] = UniquePlayer{
				PlayerID:   p.PlayerID,
				PlayerName: p.PlayerName,
				Position:   p.TeamPosition,
			}
		}
		mu.Unlock()
	})

	scheduleCollector.OnError(func(r *colly.Response, err error) {
		log.Printf("❌ [MATCHUPS] Schedule Collector Error: %v (%s)\n", err, r.Request.URL)
	})
	matchupCollector.OnError(func(r *colly.Response, err error) {
		log.Printf("❌ [MATCHUPS] Matchup Collector Error: %v (%s)\n", err, r.Request.URL)
	})

	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		startURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/schedule", cfg.LeagueID, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)
		scheduleCollector.Request("GET", startURL, nil, ctx, nil)
	}

	scheduleCollector.Wait()
	matchupCollector.Wait()

	// Sort by matchupId
	sort.Slice(allMatchups, func(i, j int) bool {
		return allMatchups[i].MatchupID < allMatchups[j].MatchupID
	})

	// Sort player stats by MatchupID and then PlayerID
	sort.Slice(allPlayerStats, func(i, j int) bool {
		if allPlayerStats[i].MatchupID != allPlayerStats[j].MatchupID {
			return allPlayerStats[i].MatchupID < allPlayerStats[j].MatchupID
		}
		return allPlayerStats[i].PlayerID < allPlayerStats[j].PlayerID
	})

	// Group matchups by year
	matchupsByYear := groupMatchupsByYear(allMatchups)
	playerStatsByYear := groupPlayerStatsByYear(allPlayerStats)
	years := getSortedMatchupsYears(matchupsByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writeMatchupsYear(year, matchupsByYear[year], playerStatsByYear[year], exportDir)
	}

	// Build, sort, and write unique players
	writeUniquePlayersFile(uniquePlayersMap, exportDir)

	fmt.Printf("\t✅ Completed matchups history scraping (took %s)\n", time.Since(startTime))
}

func writeUniquePlayersFile(uniquePlayersMap map[string]UniquePlayer, exportDir string) {
	var allUniquePlayers []UniquePlayer
	for _, up := range uniquePlayersMap {
		allUniquePlayers = append(allUniquePlayers, up)
	}
	sort.Slice(allUniquePlayers, func(i, j int) bool {
		return allUniquePlayers[i].PlayerID < allUniquePlayers[j].PlayerID
	})

	playersFilePath := fmt.Sprintf("%s/players.json", exportDir)
	playersFile, playersErr := os.Create(playersFilePath)
	if playersErr != nil {
		log.Printf("❌ [MATCHUPS] Error creating %s: %v\n", playersFilePath, playersErr)
	} else {
		defer playersFile.Close()
		playersEncoder := json.NewEncoder(playersFile)
		playersEncoder.SetIndent("", "  ")
		if err := playersEncoder.Encode(allUniquePlayers); err != nil {
			log.Printf("❌ [MATCHUPS] Error encoding unique players to JSON: %v\n", err)
		} else {
			fmt.Printf("\t✅ Successfully saved %d unique players to players.json\n", len(allUniquePlayers))
		}
	}
}

func groupMatchupsByYear(allMatchups []MatchupHistory) map[int][]MatchupHistory {
	matchupsByYear := make(map[int][]MatchupHistory)
	for _, matchup := range allMatchups {
		matchupsByYear[matchup.Year] = append(matchupsByYear[matchup.Year], matchup)
	}
	return matchupsByYear
}

func groupPlayerStatsByYear(allPlayerStats []PlayerMatchupStatistic) map[int][]PlayerMatchupStatistic {
	statsByYear := make(map[int][]PlayerMatchupStatistic)
	for _, stat := range allPlayerStats {
		statsByYear[stat.Year] = append(statsByYear[stat.Year], stat)
	}
	return statsByYear
}

func getSortedMatchupsYears(matchupsByYear map[int][]MatchupHistory) []int {
	var years []int
	for year := range matchupsByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writeMatchupsYear(year int, yearMatchups []MatchupHistory, yearPlayerStats []PlayerMatchupStatistic, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	// Write Matchups
	fileName := "matchup-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [MATCHUPS] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearMatchups); err != nil {
		log.Printf("❌ [MATCHUPS] Error encoding matchup history to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d matchups to %d/%s\n", len(yearMatchups), year, fileName)
	}

	// Write Player Stats
	statsFileName := "player-matchup-statistics-history.json"
	statsFilePath := fmt.Sprintf("%s/%s", yearDir, statsFileName)
	statsFile, statsErr := os.Create(statsFilePath)
	if statsErr != nil {
		log.Printf("❌ [MATCHUPS] Error creating %s: %v\n", statsFilePath, statsErr)
		return
	}
	defer statsFile.Close()

	statsEncoder := json.NewEncoder(statsFile)
	statsEncoder.SetIndent("", "  ")
	if err := statsEncoder.Encode(yearPlayerStats); err != nil {
		log.Printf("❌ [MATCHUPS] Error encoding player stats to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d player statistics to %d/%s\n", len(yearPlayerStats), year, statsFileName)
	}
}

func buildStatHeaders(table *colly.HTMLElement) []string {
	var headers []string
	groupMap := make(map[int]string)

	table.DOM.Find("thead tr.first th").Each(func(i int, s *goquery.Selection) {
		groupName := strings.TrimSpace(s.Find("span").Text())
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
			groupName = "misc"
		} else if strings.Contains(lowGroupName, "fum") {
			groupName = "fum"
		}

		colspanStr, _ := s.Attr("colspan")
		colspan, _ := strconv.Atoi(colspanStr)
		if colspan == 0 {
			colspan = 1
		}

		startIdx := len(groupMap)
		for j := 0; j < colspan; j++ {
			groupMap[startIdx+j] = strings.ToLower(groupName)
		}
	})

	table.DOM.Find("thead tr.last th").Each(func(i int, s *goquery.Selection) {
		if s.HasClass("stat") {
			statLabel := strings.ToLower(strings.TrimSpace(s.Find("span").Text()))
			statLabel = strings.ReplaceAll(statLabel, " ", "_")
			group := groupMap[i]

			fullName := statLabel
			if group != "" && group != "fantasy" {
				fullName = group + "_" + statLabel
			}
			headers = append(headers, fullName)
		} else {
			headers = append(headers, "")
		}
	})

	return headers
}

func mapMatchupStatToSleeper(nflKey string) string {
	switch nflKey {
	// Passing
	case "passing_yds":
		return "pass_yd"
	case "passing_td":
		return "pass_td"
	case "passing_int":
		return "pass_int"
	case "passing_sck":
		return "pass_sack"

	// Rushing
	case "rushing_att":
		return "rush_att"
	case "rushing_yds":
		return "rush_yd"
	case "rushing_td":
		return "rush_td"

	// Receiving
	case "receiving_rec":
		return "rec"
	case "receiving_yds":
		return "rec_yd"
	case "receiving_td":
		return "rec_td"

	// Kicking
	case "pat_made":
		return "xpm"
	case "pat_miss":
		return "xpmiss"
	case "fg_made_0-19":
		return "fgm_0_19"
	case "fg_made_20-29":
		return "fgm_20_29"
	case "fg_made_30-39":
		return "fgm_30_39"
	case "fg_made_40-49":
		return "fgm_40_49"
	case "fg_made_50+":
		return "fgm_50p"

	// Defense
	case "defense_sck":
		return "sack"
	case "defense_int":
		return "int"
	case "defense_fum":
		return "fum_rec"
	case "defense_safe":
		return "safe"
	case "defense_td":
		return "def_td"
	case "score_td":
		return "def_td"
	case "score_saf":
		return "safe"
	case "points_pts_allow":
		return "pts_allow"
	case "score_def_2pt_ret":
		return "def_2pt"
	case "turnover_fum_rec":
		return "fum_rec"
	case "turnover_int":
		return "int"
	case "turnover_fum_f":
		return "ff"
	case "tackles_sack":
		return "sack"
	case "yards_yds_allow":
		return "yds_allow"
	case "kick_block":
		return "blk_kick"

	// Misc
	case "fum_lost":
		return "fum_lost"
	case "return_td":
		return "st_td"
	case "ret_td":
		return "def_st_td"
	case "return_yds":
		return "kr_yd" // we use kickoff return yards since it is not possible to map otherwise
	case "misc_2pt":
		return "pass_2pt" // This is ambiguous in NFL but usually stored as 2pt
	case "misc_fumtd":
		return "fum_rec_td"
	case "points":
		return "points"
	}

	fmt.Println("Unmappable statistics: " + nflKey)
	return nflKey
}

func parseMatchupPlayerRow(e *colly.HTMLElement, statHeaders []string) MatchupPlayer {
	idStr := ""
	nameClass := e.ChildAttr(".playerNameAndInfo .playerName", "class")
	if matches := playerIDRegex.FindStringSubmatch(nameClass); len(matches) > 1 {
		idStr = matches[1]
	}

	name := e.ChildText(".playerNameAndInfo .playerName")

	rawPos := strings.TrimSpace(e.ChildText(".teamPosition"))
	startingPosition, status := utils.MapToSleeperPosition(rawPos)

	teamPositionText := e.ChildText(".playerNameAndInfo em")
	teamPosition := ""
	team := ""
	if matches := playerTeamAndPositionRegex.FindStringSubmatch(teamPositionText); len(matches) > 1 {
		teamPosition = matches[1]
		team = matches[2]
	} else {
		if strings.Contains(teamPositionText, "DEF") {
			team = utils.MapTeamAbbreviation(name)
			teamPosition = "DEF"
		} else {
			teamPosition = teamPositionText
		}
	}

	stats := make(map[string]float32)
	e.DOM.Find("td").Each(func(i int, s *goquery.Selection) {
		if i < len(statHeaders) && statHeaders[i] != "" {
			valStr := strings.TrimSpace(s.Text())
			valStr = strings.ReplaceAll(valStr, "-", "0")
			valStr = strings.ReplaceAll(valStr, ",", "")
			val, _ := strconv.ParseFloat(valStr, 32)

			sleeperKey := mapMatchupStatToSleeper(statHeaders[i])
			if sleeperKey != "" {
				stats[sleeperKey] = float32(val)
			}
		}
	})

	points := stats["points"]
	delete(stats, "points")

	return MatchupPlayer{
		PlayerID:         idStr,
		PlayerName:       name,
		Status:           status,
		StartingPosition: startingPosition,
		Team:             team,
		TeamPosition:     teamPosition,
		Points:           points,
		Stats:            stats,
	}
}
