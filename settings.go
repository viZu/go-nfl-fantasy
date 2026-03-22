package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
)

type LeagueSettings struct {
	Year            int
	RosterSettings  map[string]string
	ScoringSettings map[string]map[string]string
}

func scrapeSettings() {
	c := createColly(&colly.LimitRule{
		DomainGlob:  "*fantasy.nfl.com*",
		Parallelism: 2,
	})

	c.OnHTML(".confirmationPreview", func(e *colly.HTMLElement) {
		year := e.Request.Ctx.GetAny("year").(int)

		fmt.Printf("    [Settings] Year: %d\n", year)

		// Roster Positions
		fmt.Println("      Roster Positions:")
		e.ForEach(".positionsAndRoster li", func(_ int, el *colly.HTMLElement) {
			pos := strings.TrimSuffix(strings.TrimSpace(el.ChildText("em")), ":")
			val := strings.TrimSpace(el.ChildText(".value"))

			// Simple mapping attempt for Sleeper comparison
			sleeperPos := mapSleeperPosition(pos)
			fmt.Printf("        %-25s: %-10s (Sleeper: %s)\n", pos, val, sleeperPos)
		})

		// Scoring Settings
		fmt.Println("      Scoring Settings:")
		e.ForEach(".scoreSettings h5.settingsHeader", func(_ int, el *colly.HTMLElement) {
			category := strings.TrimSpace(el.Text)
			fmt.Printf("        Category: %s\n", category)

			nextDiv := el.DOM.NextAllFiltered("div.settingsContent").First()
			nextDiv.Find("li").Each(func(_ int, s *goquery.Selection) {
				stat := strings.TrimSuffix(strings.TrimSpace(s.Find("em").Text()), ":")
				valStr := strings.TrimSpace(s.Find(".value").Text())

				// Normalize "Yards Allowed X-Y" to "X-Y yards allowed"
				displayStat := stat
				if strings.Contains(strings.ToLower(stat), "yards allowed") {
					parts := strings.Split(strings.ToLower(stat), "yards allowed ")
					if len(parts) > 1 {
						displayStat = fmt.Sprintf("%s yards allowed", strings.TrimSpace(parts[1]))
					}
				}

				sleeperKeys := mapSleeperScoring(category, displayStat)
				parsedVal := parseNFLValue(valStr)

				fmt.Printf("          %-40s: %-25s -> %-20s | Value: %.4f\n", displayStat, valStr, strings.Join(sleeperKeys, ", "), parsedVal)
			})
		})
	})

	for year := startYear; year <= endYear; year++ {
		targetURL := fmt.Sprintf("https://fantasy.nfl.com/league/%s/history/%d/settings", leagueId, year)
		ctx := colly.NewContext()
		ctx.Put("year", year)

		fmt.Printf("Scraping league settings for %d...\n", year)
		err := c.Request("GET", targetURL, nil, ctx, nil)
		if err != nil {
			log.Printf("Error requesting settings for %d: %v", year, err)
		}
	}

	c.Wait()
}

func mapSleeperPosition(nflPos string) string {
	nflPos = strings.ToLower(nflPos)
	switch {
	case strings.Contains(nflPos, "quarterback"):
		return "QB"
	case strings.Contains(nflPos, "running back") && !strings.Contains(nflPos, "/"):
		return "RB"
	case strings.Contains(nflPos, "wide receiver") && !strings.Contains(nflPos, "/"):
		return "WR"
	case strings.Contains(nflPos, "tight end") && !strings.Contains(nflPos, "/"):
		return "TE"
	case strings.Contains(nflPos, "/") && strings.Contains(nflPos, "running") && strings.Contains(nflPos, "receiver"):
		return "WRRB_FLEX"
	case strings.Contains(nflPos, "/") && strings.Contains(nflPos, "tight") && strings.Contains(nflPos, "receiver"):
		return "REC_FLEX"
	case strings.Contains(nflPos, "/"):
		return "FLEX"
	case strings.Contains(nflPos, "kicker"):
		return "K"
	case strings.Contains(nflPos, "defensive team"):
		return "DEF"
	case strings.Contains(nflPos, "bench"):
		return "BN"
	case strings.Contains(nflPos, "reserve"):
		return "IR"
	default:
		return "UNKNOWN"
	}
}

func mapSleeperScoring(category, stat string) []string {
	cat := strings.ToLower(category)
	s := strings.ToLower(stat)

	switch {
	// Offense
	case strings.Contains(cat, "offense"):
		switch {
		case strings.Contains(s, "passing yards"):
			return []string{"pass_yd"}
		case strings.Contains(s, "passing touchdowns"):
			return []string{"pass_td"}
		case strings.Contains(s, "interceptions thrown"):
			return []string{"pass_int"}
		case strings.Contains(s, "every time sacked"):
			return []string{"pass_sack"}
		case strings.Contains(s, "rushing attempts"):
			return []string{"rush_att"}
		case strings.Contains(s, "rushing yards"):
			return []string{"rush_yd"}
		case strings.Contains(s, "rushing touchdowns"):
			return []string{"rush_td"}
		case strings.Contains(s, "receptions"):
			return []string{"rec"}
		case strings.Contains(s, "receiving yards"):
			return []string{"rec_yd"}
		case strings.Contains(s, "receiving touchdowns"):
			return []string{"rec_td"}
		case strings.Contains(s, "kickoff and punt return yards"):
			return []string{"kr_yd", "pr_yd"}
		case strings.Contains(s, "kickoff and punt return touchdowns"):
			return []string{"st_td"}
		case strings.Contains(s, "fumble recovered for td"):
			return []string{"fum_rec_td"}
		case strings.Contains(s, "fumble lost"):
			return []string{"fum_lost"}
		case strings.Contains(s, "2-point conversions"):
			return []string{"pass_2pt", "rush_2pt", "rec_2pt"}
		}

	// Kicking
	case strings.Contains(cat, "kicking"):
		switch {
		case strings.Contains(s, "pat made"):
			return []string{"xpm"}
		case strings.Contains(s, "pat missed"):
			return []string{"xpmiss"}
		case strings.Contains(s, "fg made 0-19"):
			return []string{"fgm_0_19"}
		case strings.Contains(s, "fg made 20-29"):
			return []string{"fgm_20_29"}
		case strings.Contains(s, "fg made 30-39"):
			return []string{"fgm_30_39"}
		case strings.Contains(s, "fg made 40-49"):
			return []string{"fgm_40_49"}
		case strings.Contains(s, "fg made 50+"):
			return []string{"fgm_50p"}
		}

	// Defense
	case strings.Contains(cat, "defense"):
		switch {
		case strings.Contains(s, "sacks"):
			return []string{"sack"}
		case strings.Contains(s, "interceptions"):
			return []string{"int"}
		case strings.Contains(s, "fumbles recovered"):
			return []string{"fum_rec"}
		case strings.Contains(s, "fumbles forced"):
			return []string{"ff"}
		case strings.Contains(s, "safeties"):
			return []string{"safe"}
		case strings.Contains(s, "touchdowns"):
			return []string{"def_td"}
		case strings.Contains(s, "blocked kicks"):
			return []string{"blk_kick"}
		case strings.Contains(s, "points allowed 0"):
			return []string{"pts_allow_0"}
		case strings.Contains(s, "points allowed 1-6"):
			return []string{"pts_allow_1_6"}
		case strings.Contains(s, "points allowed 7-13"):
			return []string{"pts_allow_7_13"}
		case strings.Contains(s, "points allowed 14-20"):
			return []string{"pts_allow_14_20"}
		case strings.Contains(s, "points allowed 21-27"):
			return []string{"pts_allow_21_27"}
		case strings.Contains(s, "points allowed 28-34"):
			return []string{"pts_allow_28_34"}
		case strings.Contains(s, "points allowed 35+"):
			return []string{"pts_allow_35p"}
		case strings.Contains(s, "less than 100") || strings.Contains(s, "0-99 yards allowed"):
			return []string{"yds_allow_0_100"}
		case strings.Contains(s, "100-199 yards allowed"):
			return []string{"yds_allow_100_199"}
		case strings.Contains(s, "200-299 yards allowed"):
			return []string{"yds_allow_200_299"}
		case strings.Contains(s, "300-399 yards allowed"):
			return []string{"yds_allow_300_349", "yds_allow_350_399"}
		case strings.Contains(s, "400-449 yards allowed"):
			return []string{"yds_allow_400_449"}
		case strings.Contains(s, "450-499 yards allowed"):
			return []string{"yds_allow_450_499"}
		case strings.Contains(s, "500+ yards allowed"):
			return []string{"yds_allow_500_549", "yds_allow_550p"}
		}
	}

	return []string{"UNKNOWN"}
}

func parseNFLValue(valStr string) float64 {
	valStr = strings.ToLower(valStr)

	// Handle "1 point per 25 yards"
	if strings.Contains(valStr, "point per") {
		parts := strings.Fields(valStr)
		if len(parts) >= 4 {
			pts, _ := strconv.ParseFloat(parts[0], 64)
			unit, _ := strconv.ParseFloat(parts[3], 64)
			if unit != 0 {
				return pts / unit
			}
		}
	}

	// Handle "6 points" or "-2 points"
	parts := strings.Fields(valStr)
	if len(parts) > 0 {
		ptsStr := strings.ReplaceAll(parts[0], ",", "")
		pts, _ := strconv.ParseFloat(ptsStr, 64)
		return pts
	}

	return 0
}
