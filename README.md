<p align="center">
  <a href="https://fusionproxy.net/?utm_source=github&utm_medium=referral&utm_campaign=golang-web-scraping">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/FusionProxies/.github/main/assets/logo-on-dark.png">
      <img src="https://raw.githubusercontent.com/FusionProxies/.github/main/assets/logo-on-light.png" alt="Fusion Proxy" height="52">
    </picture>
  </a>
</p>

# Web scraping with Go and Colly

A complete Go web scraper built on [Colly](https://github.com/gocolly/colly): CSS selectors, pagination, rate limiting, retries, a proxy with one sticky IP, and CSV and JSON output, in one `main.go`.

The example scrapes [books.toscrape.com](https://books.toscrape.com), a sandbox site built for scraping practice. Requires Go 1.24 or newer.

```bash
go run . -pages 3
```

```
page /catalogue/page-1.html: 50469 bytes
page /catalogue/page-2.html: 50877 bytes
page /catalogue/page-3.html: 51008 bytes
saved 60 books to books.csv and books.json
```

## How Colly works

You create a **collector**, register callbacks for the things you care about, and call `Visit`. Colly downloads pages, runs your callbacks on every element that matches a selector, and handles cookies, redirects and limits.

```go
c := colly.NewCollector(colly.AllowedDomains("books.toscrape.com"))

c.OnHTML("article.product_pod", func(e *colly.HTMLElement) {
    title := e.ChildAttr("h3 a", "title")
    price := e.ChildText("p.price_color")
    fmt.Println(title, price)
})

c.Visit("https://books.toscrape.com/catalogue/page-1.html")
```

`ChildText` and `ChildAttr` read inside the matched element; `e.Request.AbsoluteURL` turns relative links into full URLs.

## Pagination

Register a callback on the "next" link and visit it. `AllowedDomains` keeps the crawler from wandering off the site:

```go
c.OnHTML("li.next a", func(e *colly.HTMLElement) {
    e.Request.Visit(e.Attr("href"))
})
```

## Rate limiting

```go
c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 1, Delay: time.Second})
```

One request at a time, a second apart. Raise `Parallelism` (and use `colly.Async(true)` with `c.Wait()`) for bigger jobs, but keep the load on the site reasonable.

## Retries

Colly calls `OnError` for network errors and non-2xx answers. Retry the temporary ones with a growing pause:

```go
c.OnError(func(r *colly.Response, err error) {
    tries, _ := r.Ctx.GetAny("tries").(int)
    if (r.StatusCode == 0 || r.StatusCode == 429 || r.StatusCode >= 500) && tries < 3 {
        r.Ctx.Put("tries", tries+1)
        time.Sleep(time.Duration(2<<tries) * time.Second)
        r.Request.Retry()
    }
})
```

## Proxy

`c.SetProxy` takes a proxy URL with credentials. With a residential gateway, options in the username pick the country and keep one IP:

```go
session := fmt.Sprintf("%s-country-us-sessid-%d-sessTime-1800s", user, 1000000+rand.Intn(9000000))
proxy := (&url.URL{Scheme: "http", User: url.UserPassword(session, pass), Host: "resi.fusionproxy.net:12286"}).String()
c.SetProxy(proxy)
```

`url.UserPassword` escapes special characters in the password for you. The scraper does this when the variables are set:

```bash
export FUSION_USER="your_username" FUSION_PASS="your_password"
go run . -pages 3
```

To spread requests over several proxies, Colly has a round-robin switcher: `c.SetProxyFunc(proxy.RoundRobinProxySwitcher(p1, p2, p3))` from `github.com/gocolly/colly/v2/proxy`. With a rotating gateway you do not need it: drop `-sessid-...` from the username and every new connection gets a new IP.

## Output

`books.csv` and `books.json` with `title`, `price`, `rating`, `in_stock` and `url`.

## Related

- [python-web-scraping](https://github.com/FusionProxies/python-web-scraping): the same scraper in Python
- [nodejs-web-scraping](https://github.com/FusionProxies/nodejs-web-scraping): the same scraper in Node.js
- [proxy-checker](https://github.com/FusionProxies/proxy-checker): a Go CLI that checks HTTP and SOCKS proxies in bulk

[Fusion Proxy residential proxies](https://fusionproxy.net/proxies/residential?utm_source=github&utm_medium=referral&utm_campaign=golang-web-scraping) from $2.95/GB, with a [free trial](https://fusionproxy.net/free-trial?utm_source=github&utm_medium=referral&utm_campaign=golang-web-scraping).

## License

MIT, see [LICENSE](LICENSE).
