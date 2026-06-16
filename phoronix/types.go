package phoronix

// Article is the list-view record for a Phoronix article, sourced from RSS or
// search results.
type Article struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Category    string `json:"category,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Author      string `json:"author,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

// ArticleDetail is the full-content record for a Phoronix article.
type ArticleDetail struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Category    string   `json:"category,omitempty"`
	Author      string   `json:"author,omitempty"`
	PublishedAt string   `json:"published_at,omitempty"`
	Body        string   `json:"body"`
	Tags        []string `json:"tags,omitempty"`
}

// Categories is the set of known Phoronix RSS category slugs.
var Categories = []string{
	"hardware",
	"linux",
	"processors",
	"graphics",
	"storage",
	"memory",
	"network",
	"android",
	"mobile",
	"mac",
	"windows",
	"benchmark",
	"distros",
	"kernel",
}
