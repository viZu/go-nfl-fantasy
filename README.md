# go-nfl-fantasy

This project is a web scraper that connects to your NFL Fantasy league and extracts your historical league data. It serves as a reliable way to backup your entire league's history, from drafts and matchups to playoffs and final standings, ensuring your hard-earned fantasy football records are safely stored locally.

## 1. Introduction
`go-nfl-fantasy` automates the extraction of historical league data from NFL Fantasy. It navigates through various league pages (owners, standings, matchups, trades, etc.) and exports the data into structured JSON formats.

Under the hood, it heavily relies on **[Colly](https://github.com/gocolly/colly)**, a scraping framework for Go. Colly handles the asynchronous requests, rate limiting, and session management (via your browser's cookie). It is paired with **[goquery](https://github.com/PuerkitoBio/goquery)**, which parses the HTML DOM to extract the relevant data efficiently.

## 2. Data Scraped
The scraper extracts comprehensive league data, organized into several JSON structures. Here is a breakdown of what is captured:

### Managers
Captures the individuals managing the teams in your league.
- `year`: The season year.
- `managerName`: The primary manager's name.
- `userId`: The unique ID of the primary manager.
- `coManagerName`: The name of the co-manager (if applicable).
- `coUserId`: The unique ID of the co-manager (if applicable).

### Season Settings
Captures the league's rules and scoring settings for a given season.
- `year`: The season year.
- `rosterPositions`: Allowed roster positions, tracking count and max limits.
- `offenseSettings`, `kickingSettings`, `dstSettings`: Scoring configurations for different positions and actions.

### Drafts
Captures every pick made during the season's draft.
- `year`: The season year.
- `round`: The draft round.
- `pick`: The overall pick number.
- `teamId`: The ID of the team that made the pick.
- `teamName`: The name of the team.

### Rosters
Captures the final team rosters.
- `year`: The season year.
- `teamId`: The team ID.
- `players`: A list of players on the roster, including their `starterType`, `playerName`, `playerId`, `rosterPosition`, and real-life `team`.

### Standings
Captures the regular-season performance and division rankings.
- `year`: The season year.
- `divisionId` / `divisionName`: The division details.
- `teams`: A list of teams with their `teamId`, `teamName`, `divisionRank`, `overallRank`, and total `wins`.

### Playoffs
Captures the structure and schedule of the playoff bracket.
- `year`: The season year.
- `week`: The NFL week the game took place.
- `round`: The playoff round number.
- `roundLabel`: The name of the round (e.g., "Championship").
- `bracketType`: Determines if it's the championship or consolation bracket.

### End Standings
Captures the final end-of-season rank for each team after the playoffs.
- `year`: The season year.
- `rank`: The final overall placement.
- `teamId` / `teamName`: The team's identifier and name.

### Players
Captures a global mapping of all unique players encountered across all matchups. Stored in the root of the league folder as `players.json`.
- `playerId`: The unique ID of the player.
- `playerName`: The player's name.
- `position`: The player's primary NFL position.

### Matchups
Captures the weekly head-to-head game results between teams.
- `year`: The season year.
- `week`: The NFL week.
- `matchupId`: A unique identifier for the matchup.
- `team1Id` / `team2Id`: The IDs of the competing teams.
- `team1Points` / `team2Points`: The total points scored by each team.

### Player Matchup Statistics
Captures detailed individual player performances for each matchup.
- `year`: The season year.
- `matchupId`: The associated matchup.
- `teamId`: The fantasy team the player was on.
- `playerId`: The ID of the player.
- `position`: The player's starting roster position (e.g., QB, BN, FLEX).
- `nflTeam`: The player's real-life NFL team.
- `status`: The player's roster status (Starter, Bench, IR).
- `points`: Total fantasy points scored.
- `stats`: A detailed map of the player's statistical achievements.

### Trades
Captures the trades executed between teams.
- `year`: The season year.
- `transactionDate` / `transactionWeek`: When the trade occurred.
- `transactionOwnerUserId`: The user ID of the trade initiator.
- `transaction`: A list of exchanges detailing which team is sending to which (`from` / `to`) and the items being traded (`sends` array containing the `type` of player/draftPick, `playerId`, or `draftPick` details).

## 3. Usage Guide
To use the scraper, you must provide it with access to your private league via a configuration file (`.env`) or environment variables.

### Configuration File
1. **Create .env**:
   Copy the provided `.env.example` to a new file named `.env`:
   ```bash
   cp .env.example .env
   ```

2. **Set Environment Variables**:
   Open the `.env` file and configure the following:
   - `LEAGUE_ID`: The ID of your fantasy league (found in the URL of your league page).
   - `START_YEAR`: The first year you want to scrape (e.g., `2015`).
   - `END_YEAR`: The last year you want to scrape (e.g., `2023`).
   - `NFL_COOKIE`: Your authentication cookie. 

### How to get your NFL_COOKIE
To get your authentication cookie, log into [fantasy.nfl.com](https://fantasy.nfl.com) in your browser:
1. Open Developer Tools (F12) and go to the **Network** tab.
2. Refresh the page.
3. Click the main document request (usually named with your league ID).
4. Find the **Request Headers** section and copy the entire value of the `Cookie` string.

### Running the Scraper
Once you are ready, run the executable from your terminal:
```bash
./go-nfl-fantasy
```
*(If you are running from source, use `go run .`)*

The scraper will asynchronously fetch all pages across the configured years and output the JSON data into a new subdirectory named `{LEAGUE_ID}-{league-name-sanitized}/` (e.g., `123456-my-awesome-league/`) within your current working directory.

Inside this folder, data is organized into year-specific subdirectories (e.g., `2023/`, `2024/`) containing files like `settings-history.json`, `matchup-history.json`, etc. Global mapping files, such as `players.json`, are saved directly in the root of the league directory.

## 4. Building the Project
If you want to compile the binary from source, ensure you have Go 1.25 or higher installed.

Open your terminal in the project directory and run the command corresponding to your operating system:

**For macOS (Apple Silicon):**
```bash
GOOS=darwin GOARCH=arm64 go build -o go-nfl-fantasy
```

**For macOS (Intel):**
```bash
GOOS=darwin GOARCH=amd64 go build -o go-nfl-fantasy
```

**For Linux:**
```bash
GOOS=linux GOARCH=amd64 go build -o go-nfl-fantasy
```

**For Windows:**
```bash
GOOS=windows GOARCH=amd64 go build -o go-nfl-fantasy.exe
```

## 5. Used Libraries and Language
- **Language:** Go (Golang) version 1.25
- **Libraries:**
  - **[Colly](https://github.com/gocolly/colly):** (v1.2.0) The primary web scraping framework used to manage asynchronous requests, rate limits, and caching.
  - **[Goquery](https://github.com/PuerkitoBio/goquery):** (v1.11.0) Used by Colly for parsing HTML documents and querying the DOM similar to jQuery.
  - **[Godotenv](https://github.com/joho/godotenv):** (v1.5.1) Used to securely load environment variables from the `.env` file.
