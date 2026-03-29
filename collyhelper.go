package main

import (
	"time"

	"github.com/gocolly/colly"
)

func createColly(limitRule *colly.LimitRule) *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		colly.AllowedDomains("fantasy.nfl.com"),
		colly.Async(true),
	)

	// This injects your browser session into Colly
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Cookie", nflCookie)
		//fmt.Println("Visiting:", r.URL.String())
	})

	if limitRule != nil {
		c.Limit(limitRule)
	} else {
		c.Limit(&colly.LimitRule{
			DomainGlob:  "*fantasy.nfl.com*",
			Delay:       1 * time.Second,
			RandomDelay: 500 * time.Millisecond,
		})
	}

	return c
}
