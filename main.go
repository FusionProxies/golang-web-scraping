// Web scraping with Go and Colly: pagination, rate limiting, retries, proxies, CSV and JSON.
//
// Scrapes https://books.toscrape.com (a sandbox built for scraping practice).
//
//	go run . -pages 3
//	FUSION_USER=... FUSION_PASS=... go run . -pages 3     # through a residential proxy
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

type Book struct {
	Title   string  `json:"title"`
	Price   float64 `json:"price"`
	Rating  int     `json:"rating"`
	InStock bool    `json:"in_stock"`
	URL     string  `json:"url"`
}

var ratings = map[string]int{"One": 1, "Two": 2, "Three": 3, "Four": 4, "Five": 5}

func main() {
	pages := flag.Int("pages", 3, "how many listing pages to scrape (the site has 50)")
	flag.Parse()

	c := colly.NewCollector(
		colly.AllowedDomains("books.toscrape.com"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"),
	)
	c.SetRequestTimeout(30 * time.Second)
	// be polite: one request at a time with a pause between them
	if err := c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 1, Delay: time.Second}); err != nil {
		log.Fatal(err)
	}

	// optional residential proxy: one sticky IP for the whole run
	if user, pass := os.Getenv("FUSION_USER"), os.Getenv("FUSION_PASS"); user != "" && pass != "" {
		country := os.Getenv("FUSION_COUNTRY")
		if country == "" {
			country = "us"
		}
		session := fmt.Sprintf("%s-country-%s-sessid-%d-sessTime-1800s", user, country, 1000000+rand.Intn(9000000))
		proxy := (&url.URL{Scheme: "http", User: url.UserPassword(session, pass), Host: "resi.fusionproxy.net:12286"}).String()
		if err := c.SetProxy(proxy); err != nil {
			log.Fatal(err)
		}
		log.Println("using a residential proxy (sticky session)")
	}

	var books []Book
	page := 1

	c.OnHTML("article.product_pod", func(e *colly.HTMLElement) {
		price, _ := strconv.ParseFloat(strings.TrimLeft(e.ChildText("p.price_color"), "£Â"), 64)
		rating := 0
		for _, cls := range strings.Fields(e.ChildAttr("p.star-rating", "class")) {
			if r, ok := ratings[cls]; ok {
				rating = r
			}
		}
		books = append(books, Book{
			Title:   e.ChildAttr("h3 a", "title"),
			Price:   price,
			Rating:  rating,
			InStock: strings.Contains(e.ChildText("p.availability"), "In stock"),
			URL:     e.Request.AbsoluteURL(e.ChildAttr("h3 a", "href")),
		})
	})

	// follow the "next" link until the page limit
	c.OnHTML("li.next a", func(e *colly.HTMLElement) {
		if page < *pages {
			page++
			_ = e.Request.Visit(e.Attr("href"))
		}
	})

	c.OnResponse(func(r *colly.Response) { log.Printf("page %s: %d bytes", r.Request.URL.Path, len(r.Body)) })

	// retry temporary failures up to 3 times
	c.OnError(func(r *colly.Response, err error) {
		tries, _ := r.Ctx.GetAny("tries").(int)
		if (r.StatusCode == 0 || r.StatusCode == 429 || r.StatusCode >= 500) && tries < 3 {
			r.Ctx.Put("tries", tries+1)
			log.Printf("retrying %s (%v)", r.Request.URL, err)
			time.Sleep(time.Duration(2<<tries) * time.Second)
			_ = r.Request.Retry()
			return
		}
		log.Printf("failed %s: %v", r.Request.URL, err)
	})

	if err := c.Visit("https://books.toscrape.com/catalogue/page-1.html"); err != nil {
		log.Fatal(err)
	}
	c.Wait()

	if err := save(books); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("saved %d books to books.csv and books.json\n", len(books))
}

func save(books []Book) error {
	f, err := os.Create("books.csv")
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"title", "price", "rating", "in_stock", "url"})
	for _, b := range books {
		_ = w.Write([]string{b.Title, strconv.FormatFloat(b.Price, 'f', 2, 64), strconv.Itoa(b.Rating), strconv.FormatBool(b.InStock), b.URL})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(books, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("books.json", data, 0o644)
}
