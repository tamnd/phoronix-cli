package phoronix_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/phoronix-cli/phoronix"
)

func newTestClient(t *testing.T, srv *httptest.Server) *phoronix.Client {
	t.Helper()
	cfg := phoronix.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Timeout = 5 * time.Second
	return phoronix.NewClient(cfg)
}

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Phoronix</title>
    <item>
      <title>Linux 6.13 Brings AMD Power Improvements</title>
      <link>https://www.phoronix.com/news/Linux-613-AMD-Power</link>
      <description>&lt;p&gt;The Linux 6.13 kernel is seeing improvements to AMD power management...&lt;/p&gt;</description>
      <pubDate>Wed, 15 Jan 2025 10:00:00 +0000</pubDate>
      <author>Michael Larabel</author>
    </item>
    <item>
      <title>Mesa 25.0 RC1 Released</title>
      <link>https://www.phoronix.com/news/Mesa-25-0-RC1</link>
      <description>&lt;p&gt;Mesa 25.0 RC1 has been released...&lt;/p&gt;</description>
      <pubDate>Tue, 14 Jan 2025 09:00:00 +0000</pubDate>
      <author>Michael Larabel</author>
    </item>
  </channel>
</rss>`

func TestGetRSS_all(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rss.php" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	articles, err := c.GetRSS(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2", len(articles))
	}
	a := articles[0]
	if a.Slug != "Linux-613-AMD-Power" {
		t.Errorf("Slug = %q, want Linux-613-AMD-Power", a.Slug)
	}
	if a.Title != "Linux 6.13 Brings AMD Power Improvements" {
		t.Errorf("Title = %q", a.Title)
	}
	if a.Author != "Michael Larabel" {
		t.Errorf("Author = %q, want Michael Larabel", a.Author)
	}
	if a.PublishedAt == "" {
		t.Error("PublishedAt is empty")
	}
	if a.Summary == "" {
		t.Error("Summary is empty")
	}
}

func TestGetRSS_withCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cat := r.URL.Query().Get("category")
		if cat != "processors" {
			t.Errorf("expected category=processors, got %q", cat)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	articles, err := c.GetRSS(context.Background(), "processors")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2", len(articles))
	}
	// category should be filled from the request parameter
	if articles[0].Category != "processors" {
		t.Errorf("Category = %q, want processors", articles[0].Category)
	}
}

func TestGetRSS_empty(t *testing.T) {
	emptyRSS := `<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(emptyRSS))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	articles, err := c.GetRSS(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 0 {
		t.Errorf("got %d articles, want 0", len(articles))
	}
}

func TestGetRSS_malformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not xml at all"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetRSS(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

const sampleArticleHTML = `<!DOCTYPE html>
<html>
<head><title>Linux 6.13 AMD Power - Phoronix</title></head>
<body>
<h1 class="title">Linux 6.13 Brings AMD Power Improvements</h1>
<time>15 January 2025</time>
<div class="article-body">
  <p>The Linux 6.13 kernel is seeing improvements to AMD power management.
  These changes help reduce idle power consumption on AMD platforms significantly.</p>
  <p>Michael Larabel has been tracking these patches through the linux-next tree.</p>
</div>
</body>
</html>`

func TestGetArticle_ok(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/news/Linux-613-AMD-Power" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(sampleArticleHTML))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	detail, err := c.GetArticle(context.Background(), "Linux-613-AMD-Power")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Slug != "Linux-613-AMD-Power" {
		t.Errorf("Slug = %q, want Linux-613-AMD-Power", detail.Slug)
	}
	if detail.Title == "" {
		t.Error("Title is empty")
	}
	if detail.Body == "" {
		t.Error("Body is empty")
	}
	if detail.URL == "" {
		t.Error("URL is empty")
	}
}

func TestGetArticle_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetArticle(context.Background(), "nonexistent-slug")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

const sampleSearchHTML = `<!DOCTYPE html>
<html>
<body>
<a href="/news/Linux-613-AMD-Power">Linux 6.13 AMD Power</a>
<a href="/news/Mesa-25-0-RC1">Mesa 25.0 RC1</a>
<a href="/about">About</a>
</body>
</html>`

func TestSearch_ok(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleSearchHTML))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	articles, err := c.Search(context.Background(), "linux amd", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2", len(articles))
	}
	if articles[0].Slug != "Linux-613-AMD-Power" {
		t.Errorf("first slug = %q, want Linux-613-AMD-Power", articles[0].Slug)
	}
}

func TestSearch_noResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>No results found.</body></html>`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	articles, err := c.Search(context.Background(), "zzznomatch", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(articles) != 0 {
		t.Errorf("got %d articles, want 0", len(articles))
	}
}

func TestSlugFromInput(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Linux-613-AMD-Power", "Linux-613-AMD-Power"},
		{"https://www.phoronix.com/news/Linux-613-AMD-Power", "Linux-613-AMD-Power"},
		{"  Mesa-25-0-RC1  ", "Mesa-25-0-RC1"},
	}
	for _, tc := range cases {
		got := phoronix.SlugFromInput(tc.in)
		if got != tc.want {
			t.Errorf("SlugFromInput(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
