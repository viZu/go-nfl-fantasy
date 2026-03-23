package main

import (
	"strings"
)

func mapToSleeperPosition(pos string) string {
	pos = strings.TrimSpace(pos)
	if pos == "W/T" {
		return "REC_FLEX"
	}
	if pos == "W/R" {
		return "WRRB_FLEX"
	}
	if pos == "R/W/T" {
		return "FLEX"
	}
	if pos == "Q/R/W/T" || pos == "SuperFlex" {
		return "SUPER_FLEX"
	}
	if strings.Contains(pos, "/") || strings.Contains(pos, "\\") {
		return "FLEX"
	}
	return pos
}

func mapTeamAbbreviation(teamName string) string {
	switch teamName {
	case "Seattle Seahawks":
		return "SEA"
	case "Houston Texans":
		return "HOU"
	case "Denver Broncos":
		return "DEN"
	case "Jacksonville Jaguars":
		return "JAX"
	case "Cleveland Browns":
		return "CLE"
	case "Minnesota Vikings":
		return "MIN"
	case "Philadelphia Eagles":
		return "PHI"
	case "Los Angeles Rams":
		return "LAR"
	case "New England Patriots":
		return "NE"
	case "Buffalo Bills":
		return "BUF"
	case "New Orleans Saints":
		return "NO"
	case "Pittsburgh Steelers":
		return "PIT"
	case "Los Angeles Chargers":
		return "LAC"
	case "Atlanta Falcons":
		return "ATL"
	case "Chicago Bears":
		return "CHI"
	case "Kansas City Chiefs":
		return "KC"
	case "Carolina Panthers":
		return "CAR"
	case "Tampa Bay Buccaneers":
		return "TB"
	case "Detroit Lions":
		return "DET"
	case "Baltimore Ravens":
		return "BAL"
	case "Indianapolis Colts":
		return "IND"
	case "Green Bay Packers":
		return "GB"
	case "Miami Dolphins":
		return "MIA"
	case "Las Vegas Raiders":
		return "LV"
	case "San Francisco 49ers":
		return "SF"
	case "Tennessee Titans":
		return "TEN"
	case "Arizona Cardinals":
		return "ARI"
	case "New York Giants":
		return "NYG"
	case "Cincinnati Bengals":
		return "CIN"
	case "Washington Commanders":
		return "WAS"
	case "Dallas Cowboys":
		return "DAL"
	case "New York Jets":
		return "NYJ"
	default:
		return teamName
	}
}
