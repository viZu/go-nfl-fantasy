package main

// Data to scrape for each season
// - Regular Season
// --- Standings
// --- Matchups
// - Playoffs
// --- Standings
// --- Matchups
// - Matchups
// --- All matchups regardless of playoff/regular season
// --- Per week
// - Draft Results
// - Teams/Managers
// --- Image
// --- Season End Roster

func main() {
	//scrapeManagers()
	scrapeDrafts()
	//scrapeMatchups()
}

type WinLossRecord struct {
	Wins   int
	Losses int
	Draws  int
}

type RegularSeasonTeam struct {
	TotalRank     int
	DivisionRank  int
	Record        WinLossRecord
	PointsFor     float32
	PointsAgainst float32
}

type RegularSeasonDivision struct {
	SortOrder int
	Name      string
	Teams     []RegularSeasonTeam
}

type RegularSeasonResults struct {
	Year      int
	Divisions []RegularSeasonDivision
}
