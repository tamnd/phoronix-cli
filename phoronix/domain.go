package phoronix

import (
	"context"
	"errors"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Phoronix driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme and identity for the standalone binary and multi-domain host.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "phoronix",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "phoronix",
			Short:  "Read Phoronix Linux hardware benchmarks and news.",
			Long: `phoronix reads Linux hardware benchmark results and open-source tech
news from phoronix.com via RSS and article HTML, shapes it into clean records,
and prints output that pipes into the rest of your tools.

Quick start:
  phoronix latest
  phoronix latest --category processors
  phoronix article Linux-613-AMD-Power
  phoronix search "AMD Radeon RX 9070"`,
			Site: Host,
			Repo: "https://github.com/tamnd/phoronix-cli",
		},
	}
}

// Register installs the client factory and operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "latest",
		Group:   "articles",
		Summary: "List recent articles from the RSS feed",
		Args:    []kit.Arg{},
	}, latestOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "article",
		Group:   "articles",
		Single:  true,
		Summary: "Fetch the full content of an article",
		Args:    []kit.Arg{{Name: "slug", Help: "article slug or full URL"}},
	}, articleOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "articles",
		Summary: "Search Phoronix articles",
		Args:    []kit.Arg{{Name: "query", Help: "search terms"}},
	}, searchOp)
}

// newClient builds a Client from the kit Config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type latestInput struct {
	Category string  `kit:"flag"         help:"category slug (hardware, linux, processors, graphics, storage...)"`
	Limit    int     `kit:"flag,inherit" help:"max articles" default:"20"`
	Client   *Client `kit:"inject"`
}

type articleInput struct {
	Slug   string  `kit:"arg"  help:"article slug or full URL"`
	Client *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg"          help:"search query"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"20"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func latestOp(ctx context.Context, in latestInput, emit func(*Article) error) error {
	articles, err := in.Client.GetRSS(ctx, in.Category)
	if err != nil {
		return mapErr(err)
	}
	for i := range articles {
		if in.Limit > 0 && i >= in.Limit {
			break
		}
		if err := emit(&articles[i]); err != nil {
			return err
		}
	}
	return nil
}

func articleOp(ctx context.Context, in articleInput, emit func(*ArticleDetail) error) error {
	slug := SlugFromInput(in.Slug)
	detail, err := in.Client.GetArticle(ctx, slug)
	if err != nil {
		return mapErr(err)
	}
	return emit(detail)
}

func searchOp(ctx context.Context, in searchInput, emit func(*Article) error) error {
	articles, err := in.Client.Search(ctx, in.Query, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range articles {
		if err := emit(&articles[i]); err != nil {
			return err
		}
	}
	return nil
}

// mapErr converts library errors to kit error kinds with the right exit code.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("%s", err.Error())
	}
	if errors.Is(err, ErrBlocked) {
		return errs.RateLimited("%s", err.Error())
	}
	return err
}
