package main

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	leagueId  string
	startYear int
	endYear   int
	nflCookie string
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}

	nflCookie = os.Getenv("NFL_COOKIE")
	if nflCookie == "" {
		log.Fatal("NFL_COOKIE is not set — add it to .env or your environment")
	}

	leagueId = os.Getenv("LEAGUE_ID")
	if leagueId == "" {
		log.Fatal("LEAGUE_ID is not set — add it to .env or your environment")
	}

	sYearStr := os.Getenv("START_YEAR")
	if sYearStr == "" {
		log.Fatal("START_YEAR is not set — add it to .env or your environment")
	}
	sYear, err := strconv.Atoi(sYearStr)
	if err != nil {
		log.Fatalf("Invalid START_YEAR '%s': must be an integer", sYearStr)
	}
	startYear = sYear

	eYearStr := os.Getenv("END_YEAR")
	if eYearStr == "" {
		log.Fatal("END_YEAR is not set — add it to .env or your environment")
	}
	eYear, err := strconv.Atoi(eYearStr)
	if err != nil {
		log.Fatalf("Invalid END_YEAR '%s': must be an integer", eYearStr)
	}
	endYear = eYear

	if startYear > endYear {
		log.Fatalf("START_YEAR (%d) cannot be greater than END_YEAR (%d)", startYear, endYear)
	}
}
