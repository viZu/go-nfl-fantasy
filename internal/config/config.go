package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	LeagueID   string
	LeagueName string
	StartYear  int
	EndYear    int
	NFLCookie  string
}

func (c *Config) SanitizedLeagueName() string {
	return strings.ReplaceAll(strings.ToLower(c.LeagueName), " ", "-")
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found. Relying on environment variables.")
	}

	cfg := &Config{}

	cfg.NFLCookie = os.Getenv("NFL_COOKIE")
	if cfg.NFLCookie == "" {
		log.Fatal("NFL_COOKIE environment variable is required.")
	}

	cfg.LeagueID = os.Getenv("LEAGUE_ID")
	if cfg.LeagueID == "" {
		log.Fatal("LEAGUE_ID environment variable is required.")
	} else if _, err := strconv.Atoi(cfg.LeagueID); err != nil {
		log.Fatalf("Invalid LEAGUE_ID '%s': must be an integer", cfg.LeagueID)
	}

	sYearStr := os.Getenv("START_YEAR")
	if sYearStr == "" {
		log.Fatal("START_YEAR environment variable is required.")
	} else {
		sYear, err := strconv.Atoi(sYearStr)
		if err != nil {
			log.Fatalf("Invalid START_YEAR '%s': must be an integer", sYearStr)
		}
		cfg.StartYear = sYear
	}

	eYearStr := os.Getenv("END_YEAR")
	if eYearStr == "" {
		log.Fatal("END_YEAR environment variable is required.")
	} else {
		eYear, err := strconv.Atoi(eYearStr)
		if err != nil {
			log.Fatalf("Invalid END_YEAR '%s': must be an integer", eYearStr)
		}
		cfg.EndYear = eYear
	}

	if cfg.StartYear > cfg.EndYear {
		log.Fatalf("START_YEAR (%d) cannot be greater than END_YEAR (%d)", cfg.StartYear, cfg.EndYear)
	}

	return cfg
}
