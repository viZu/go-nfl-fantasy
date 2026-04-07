package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var (
	leagueId  string
	startYear int
	endYear   int
	nflCookie string
)

func promptInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptInt(prompt string) int {
	for {
		text := promptInput(prompt)
		val, err := strconv.Atoi(text)
		if err == nil {
			return val
		}
		fmt.Println("Invalid input. Please enter an integer.")
	}
}

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found. Proceeding with interactive input.")
	}

	nflCookie = os.Getenv("NFL_COOKIE")
	if nflCookie == "" {
		nflCookie = promptInput("Enter fantasy.nfl.com cookie value (NFL_COOKIE): ")
	}

	leagueId = os.Getenv("LEAGUE_ID")
	if leagueId == "" {
		for {
			leagueId = promptInput("Enter league ID: ")
			if _, err := strconv.Atoi(leagueId); err == nil {
				break
			}
			fmt.Println("Invalid input. Please enter an integer for league ID.")
		}
	} else {
		if _, err := strconv.Atoi(leagueId); err != nil {
			log.Fatalf("Invalid LEAGUE_ID '%s' in .env: must be an integer", leagueId)
		}
	}

	sYearStr := os.Getenv("START_YEAR")
	if sYearStr == "" {
		startYear = promptInt("Enter start year: ")
	} else {
		sYear, err := strconv.Atoi(sYearStr)
		if err != nil {
			log.Fatalf("Invalid START_YEAR '%s': must be an integer", sYearStr)
		}
		startYear = sYear
	}

	eYearStr := os.Getenv("END_YEAR")
	if eYearStr == "" {
		endYear = promptInt("Enter end year: ")
	} else {
		eYear, err := strconv.Atoi(eYearStr)
		if err != nil {
			log.Fatalf("Invalid END_YEAR '%s': must be an integer", eYearStr)
		}
		endYear = eYear
	}

	if startYear > endYear {
		log.Fatalf("START_YEAR (%d) cannot be greater than END_YEAR (%d)", startYear, endYear)
	}
}
