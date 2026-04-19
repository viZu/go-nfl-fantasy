package scraper

import (
	"encoding/json"
	"fmt"
	"gonflfantasy/internal/config"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

type TradeTransaction struct {
	Year                   int        `json:"year"`
	TransactionDate        string     `json:"transactionDate"`
	TransactionWeek        int        `json:"transactionWeek"`
	TransactionOwnerUserId string     `json:"transactionOwnerUserId"`
	Transaction            []Exchange `json:"transaction"`

	// Internal fields for grouping and sorting
	transactionId string
	parsedDate    time.Time
}

type Exchange struct {
	From  string      `json:"from"`
	To    string      `json:"to"`
	Sends []TradeItem `json:"sends"`
}

type TradeItem struct {
	Type      string         `json:"type"` // "player" or "draftPick"
	PlayerID  string         `json:"playerId,omitempty"`
	DraftPick *DraftPickInfo `json:"draftPick,omitempty"`
}

type DraftPickInfo struct {
	Year  int `json:"year"`
	Round int `json:"round"`
}

var (
	tradeClassRegex = regexp.MustCompile(`transaction-trade-(\d+)-(\d+)`)
	userIDRegex     = regexp.MustCompile(`userId-(\d+)`)
	draftPickRegex  = regexp.MustCompile(`Draft Pick - (\d{4}) Rd (\d+)`)
)

func ScrapeTrades(cfg *config.Config) {
	startTime := time.Now()
	fmt.Println("[TRADES] Starting trades history scraper...")

	c := CreateColly(cfg, &colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	var mu sync.Mutex
	tradeMap := make(map[string]*TradeTransaction)

	c.OnHTML(".tableType-transaction tbody tr", func(e *colly.HTMLElement) {
		classAttr := e.Attr("class")
		matches := tradeClassRegex.FindStringSubmatch(classAttr)
		if len(matches) < 2 {
			return
		}

		transactionId := matches[1]
		year := e.Request.Ctx.GetAny("year").(int)
		key := fmt.Sprintf("%d-%s", year, transactionId)

		mu.Lock()
		trade, exists := tradeMap[key]
		if !exists {
			dateStr := e.ChildText(".transactionDate")
			weekStr := e.ChildText(".transactionWeek")
			week, _ := strconv.Atoi(weekStr)

			ownerClass := e.ChildAttr(".transactionOwner span.userName", "class")
			ownerId := ""
			if m := userIDRegex.FindStringSubmatch(ownerClass); len(m) > 1 {
				ownerId = m[1]
			}

			parsedDate := parseTradeDate(dateStr, year)

			trade = &TradeTransaction{
				Year:                   year,
				TransactionDate:        parsedDate.Format("2006-01-02T15:04:05"),
				TransactionWeek:        week,
				TransactionOwnerUserId: ownerId,
				Transaction:            []Exchange{},
				transactionId:          transactionId,
				parsedDate:             parsedDate,
			}
			tradeMap[key] = trade
		}

		// Parse this row's exchange
		fromTeamId := extractTeamId(e.ChildAttr(".transactionFrom a", "href"))
		toTeamId := extractTeamId(e.ChildAttr(".transactionTo a", "href"))

		sends := []TradeItem{}
		e.ForEach(".playerNameAndInfo ul li", func(_ int, el *colly.HTMLElement) {
			item := TradeItem{}

			// Check for Player
			playerLink := el.DOM.Find("a.playerCard")
			if playerLink.Length() > 0 {
				item.Type = "player"
				pClass, _ := playerLink.Attr("class")
				if m := playerIDRegex.FindStringSubmatch(pClass); len(m) > 1 {
					item.PlayerID = m[1]
				}
			} else {
				// Check for Draft Pick
				text := strings.TrimSpace(el.Text)
				if m := draftPickRegex.FindStringSubmatch(text); len(m) > 2 {
					item.Type = "draftPick"
					dYear, _ := strconv.Atoi(m[1])
					dRound, _ := strconv.Atoi(m[2])
					item.DraftPick = &DraftPickInfo{
						Year:  dYear,
						Round: dRound,
					}
				}
			}

			if item.Type != "" {
				sends = append(sends, item)
			}
		})

		trade.Transaction = append(trade.Transaction, Exchange{
			From:  fromTeamId,
			To:    toTeamId,
			Sends: sends,
		})
		mu.Unlock()
	})

	for year := cfg.StartYear; year <= cfg.EndYear; year++ {
		fmt.Printf("\tProcessing year %d...\n", year)
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/transactions?transactionType=trade", cfg.LeagueID, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("❌ [TRADES] Error requesting trades for %d: %v", year, err)
		}
	}

	c.Wait()

	// Flatten map to slice
	allTrades := make([]TradeTransaction, 0, len(tradeMap))
	for _, t := range tradeMap {
		allTrades = append(allTrades, *t)
	}

	// Sort by Year (ASC) and then by parsedDate (ASC)
	sort.Slice(allTrades, func(i, j int) bool {
		if allTrades[i].Year != allTrades[j].Year {
			return allTrades[i].Year < allTrades[j].Year
		}
		return allTrades[i].parsedDate.Before(allTrades[j].parsedDate)
	})

	// Group trades by year
	tradesByYear := groupTradesByYear(allTrades)
	years := getSortedTradesYears(tradesByYear)

	// Write to JSON file per year
	exportDir := fmt.Sprintf("%s-%s", cfg.LeagueID, cfg.SanitizedLeagueName())
	os.MkdirAll(exportDir, 0755)

	for _, year := range years {
		writeTradesYear(year, tradesByYear[year], exportDir)
	}
	fmt.Printf("\t✅ Completed trades history scraping (took %s)\n", time.Since(startTime))
}

func groupTradesByYear(allTrades []TradeTransaction) map[int][]TradeTransaction {
	tradesByYear := make(map[int][]TradeTransaction)
	for _, trade := range allTrades {
		tradesByYear[trade.Year] = append(tradesByYear[trade.Year], trade)
	}
	return tradesByYear
}

func getSortedTradesYears(tradesByYear map[int][]TradeTransaction) []int {
	var years []int
	for year := range tradesByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	return years
}

func writeTradesYear(year int, yearTrades []TradeTransaction, exportDir string) {
	yearDir := fmt.Sprintf("%s/%d", exportDir, year)
	os.MkdirAll(yearDir, 0755)

	fileName := "trade-history.json"
	filePath := fmt.Sprintf("%s/%s", yearDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ [TRADES] Error creating %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(yearTrades); err != nil {
		log.Printf("❌ [TRADES] Error encoding trades to JSON for year %d: %v\n", year, err)
	} else {
		fmt.Printf("\t✅ Successfully saved %d trades to %d/%s\n", len(yearTrades), year, fileName)
	}
}

func parseTradeDate(dateStr string, year int) time.Time {
	// Handle cases like "Dec 7, Dec 7, 1:26pm" or "Oct 30, 12:28am"
	parts := strings.Split(dateStr, ",")

	monthDay := ""
	timePart := ""

	if len(parts) >= 2 {
		monthDay = strings.TrimSpace(parts[0])
		timePart = strings.TrimSpace(parts[len(parts)-1])
	}

	// Layout: Jan 2 2006 3:04pm
	layout := "Jan 2 2006 3:04pm"
	fullStr := fmt.Sprintf("%s %d %s", monthDay, year, timePart)

	t, err := time.Parse(layout, fullStr)
	if err != nil {
		log.Printf("❌ [TRADES] Error parsing date '%s': %v", fullStr, err)
		return time.Time{}
	}
	return t
}

func extractTeamId(href string) string {
	if href == "" {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return u.Query().Get("teamId")
}
