package config

import (
	"bufio"
	"fmt"
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

func (c *Config) SanitizedLeagueName() string {
	return strings.ReplaceAll(strings.ToLower(c.LeagueName), " ", "-")
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found. Proceeding with interactive input.")
	}

	cfg := &Config{}

	cfg.NFLCookie = os.Getenv("NFL_COOKIE")
	if cfg.NFLCookie == "" {
		cfg.NFLCookie = promptInput("Enter fantasy.nfl.com cookie value (NFL_COOKIE): ")
	}

	cfg.LeagueID = os.Getenv("LEAGUE_ID")
	if cfg.LeagueID == "" {
		for {
			cfg.LeagueID = promptInput("Enter league ID: ")
			if _, err := strconv.Atoi(cfg.LeagueID); err == nil {
				break
			}
			fmt.Println("Invalid input. Please enter an integer for league ID.")
		}
	} else {
		if _, err := strconv.Atoi(cfg.LeagueID); err != nil {
			log.Fatalf("Invalid LEAGUE_ID '%s' in .env: must be an integer", cfg.LeagueID)
		}
	}

	sYearStr := os.Getenv("START_YEAR")
	if sYearStr == "" {
		cfg.StartYear = promptInt("Enter start year: ")
	} else {
		sYear, err := strconv.Atoi(sYearStr)
		if err != nil {
			log.Fatalf("Invalid START_YEAR '%s': must be an integer", sYearStr)
		}
		cfg.StartYear = sYear
	}

	eYearStr := os.Getenv("END_YEAR")
	if eYearStr == "" {
		cfg.EndYear = promptInt("Enter end year: ")
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
