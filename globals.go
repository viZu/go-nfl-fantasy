package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

const leagueId = "192834"
const startYear = 2024
const endYear = 2024

var nflCookie string

func init() {
	godotenv.Load()
	nflCookie = os.Getenv("NFL_COOKIE")
	if nflCookie == "" {
		log.Fatal("NFL_COOKIE is not set — add it to .env or your environment")
	}
}
