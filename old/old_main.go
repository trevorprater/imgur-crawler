package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/gocrawl"
	"github.com/PuerkitoBio/goquery"
)

type Ext struct {
	*gocrawl.DefaultExtender
}

func (e *Ext) Visit(ctx *gocrawl.URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {
	fmt.Printf("Visit: %s\n", ctx.URL())
	doc.Find("img").Each(func(_ int, node *goquery.Selection) {
		for _, n := range node.Nodes {
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					_url, err := url.Parse(attr.Val)
					if err != nil {
						fmt.Println("COULD NOT PARSE: " + attr.Val)
						fmt.Println(err)
					} else {
						if _url.IsAbs() {
							fmt.Println(attr.Val)
						} else {
							resolvedRef := ctx.URL().ResolveReference(_url)
							fmt.Println(resolvedRef.String())
						}
					}
					continue
				}
			}
		}

	})

	return nil, true
}

func (e *Ext) Filter(ctx *gocrawl.URLContext, isVisited bool) bool {
	if isVisited {
		return false
	}
	return true
}

func main() {
	ext := &Ext{&gocrawl.DefaultExtender{}}
	// Set custom options
	opts := gocrawl.NewOptions(ext)
	opts.CrawlDelay = 5 * time.Second
	opts.LogFlags = gocrawl.LogNone
	opts.SameHostOnly = false
	opts.MaxVisits = 20
	opts.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36"

	c := gocrawl.NewCrawlerWithOptions(opts)
	c.Run("https://duckduckgo.com")
}
