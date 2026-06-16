// Package phoronix is the library behind the phoronix command line:
// the HTTP client, RSS parser, article HTML parser, and typed data models for
// phoronix.com.
//
// Phoronix publishes Linux hardware benchmarks and open-source tech news.
// The site is behind Cloudflare, but RSS feeds and article pages respond to
// standard browser User-Agent requests. This package reads the RSS feed for
// listing and the article HTML for full content.
package phoronix

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "www.phoronix.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// DefaultUserAgent is the User-Agent sent with every request. A Firefox on
// Linux UA is natural for a Linux-focused site and avoids Cloudflare bot
// classification.
const DefaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0"

// ErrBlocked is returned when Cloudflare or another challenge blocks the request.
var ErrBlocked = errors.New("phoronix: request blocked (Cloudflare challenge)")

// ErrNotFound is returned when a page returns 404.
var ErrNotFound = errors.New("phoronix: not found")

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for polite scraping.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to phoronix.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// GetRSS fetches the RSS feed for the given category. An empty category string
// fetches the main feed.
func (c *Client) GetRSS(ctx context.Context, category string) ([]Article, error) {
	u := c.cfg.BaseURL + "/rss.php"
	if category != "" {
		u += "?category=" + url.QueryEscape(category)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseRSS(body, category)
}

// GetArticle fetches the full content of an article by slug.
func (c *Client) GetArticle(ctx context.Context, slug string) (*ArticleDetail, error) {
	u := c.cfg.BaseURL + "/news/" + slug
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseArticle(body, slug, u)
}

// Search fetches search results from Phoronix for the given query.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Article, error) {
	q := strings.ReplaceAll(query, " ", "+")
	u := c.cfg.BaseURL + "/search/" + url.PathEscape(q)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	results := parseSearch(body)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// get fetches url and returns the response body. It paces and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, ErrNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}

	if isBlocked(b) {
		return nil, false, ErrBlocked
	}
	return b, false, nil
}

// isBlocked detects Cloudflare challenge pages.
func isBlocked(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "Just a moment") ||
		strings.Contains(s, "Enable JavaScript and cookies") ||
		strings.Contains(s, "cf-browser-verification")
}

// pace blocks until at least Rate has passed since the last request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- RSS parsing ---

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	Desc    string `xml:"description"`
	PubDate string `xml:"pubDate"`
	Author  string `xml:"author"`
	GUID    string `xml:"guid"`
}

func parseRSS(body []byte, category string) ([]Article, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse RSS: %w", err)
	}
	out := make([]Article, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		slug := slugFromURL(it.Link)
		pub := parsePubDate(it.PubDate)
		author := cleanAuthor(it.Author)
		summary := stripHTMLTags(it.Desc)
		cat := category
		if cat == "" {
			cat = guessCategory(it.Link)
		}
		out = append(out, Article{
			Slug:        slug,
			Title:       strings.TrimSpace(it.Title),
			URL:         it.Link,
			Category:    cat,
			Summary:     summary,
			Author:      author,
			PublishedAt: pub,
		})
	}
	return out, nil
}

// slugFromURL extracts the last path segment from a Phoronix article URL.
func slugFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return rawURL
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	return parts[len(parts)-1]
}

// guessCategory reads a rough category from the URL (e.g., /news/ means general).
func guessCategory(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

// parsePubDate converts an RSS pubDate string to RFC3339 date.
func parsePubDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return s
}

// cleanAuthor strips email addresses from author fields (RSS often has "name <email>").
func cleanAuthor(s string) string {
	s = strings.TrimSpace(s)
	// strip "<email>" suffix
	if idx := strings.Index(s, "<"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	// if it's just an email, strip it
	if strings.Contains(s, "@") {
		return ""
	}
	return s
}

// --- article HTML parsing ---

var (
	h1RE      = regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`)
	titleTagRE = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	tagRE     = regexp.MustCompile(`(?is)<[^>]+>`)
	pTagRE    = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	brRE      = regexp.MustCompile(`(?i)<br\s*/?>`)
	timeTagRE = regexp.MustCompile(`(?i)<time[^>]*>([^<]+)</time>`)
	authorRE  = regexp.MustCompile(`(?i)author[^>]*>([^<]+)<`)
)

func parseArticle(body []byte, slug, rawURL string) (*ArticleDetail, error) {
	s := string(body)

	title := ""
	if m := h1RE.FindStringSubmatch(s); len(m) > 1 {
		title = stripHTMLTags(m[1])
	}
	if title == "" {
		if m := titleTagRE.FindStringSubmatch(s); len(m) > 1 {
			title = stripHTMLTags(m[1])
			// strip site name suffix
			if i := strings.Index(title, " | "); i > 0 {
				title = strings.TrimSpace(title[:i])
			}
			if i := strings.Index(title, " - Phoronix"); i > 0 {
				title = strings.TrimSpace(title[:i])
			}
		}
	}

	pub := ""
	if m := timeTagRE.FindStringSubmatch(s); len(m) > 1 {
		pub = parsePubDate(strings.TrimSpace(m[1]))
	}

	author := ""
	if m := authorRE.FindStringSubmatch(s); len(m) > 1 {
		author = strings.TrimSpace(m[1])
	}

	body2 := extractArticleBody(s)

	return &ArticleDetail{
		Slug:        slug,
		Title:       title,
		URL:         rawURL,
		Author:      author,
		PublishedAt: pub,
		Body:        body2,
	}, nil
}

// extractArticleBody extracts the main article text from the HTML.
func extractArticleBody(s string) string {
	// Try to find article body div
	bodyDivRE := regexp.MustCompile(`(?is)<div[^>]+class="[^"]*article[^"]*"[^>]*>(.*?)</div>`)
	if m := bodyDivRE.FindStringSubmatch(s); len(m) > 1 {
		text := htmlToText(m[1])
		if len(text) > 100 {
			return text
		}
	}

	// Fallback: collect all <p> tags with meaningful text
	var paras []string
	for _, m := range pTagRE.FindAllStringSubmatch(s, -1) {
		if len(m) < 2 {
			continue
		}
		text := stripHTMLTags(m[1])
		text = strings.TrimSpace(text)
		if len(text) >= 50 {
			paras = append(paras, text)
		}
	}
	return strings.Join(paras, "\n\n")
}

// htmlToText converts HTML to plain text with paragraph breaks.
func htmlToText(s string) string {
	s = brRE.ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`(?i)</?p[^>]*>`).ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n\n")
}

// stripHTMLTags removes all HTML tags from s and unescapes entities.
func stripHTMLTags(s string) string {
	s = tagRE.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// --- search parsing ---

var newsLinkRE = regexp.MustCompile(`(?i)href="(/news/([^"?#]+))"`)

func parseSearch(body []byte) []Article {
	s := string(body)
	var out []Article
	seen := map[string]bool{}
	for _, m := range newsLinkRE.FindAllStringSubmatch(s, -1) {
		if len(m) < 3 {
			continue
		}
		slug := m[2]
		if seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, Article{
			Slug: slug,
			URL:  BaseURL + m[1],
		})
	}
	return out
}

// SlugFromInput converts a full Phoronix URL or a bare slug into a slug.
func SlugFromInput(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "http") {
		return slugFromURL(s)
	}
	return s
}
