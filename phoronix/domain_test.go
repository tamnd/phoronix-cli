package phoronix

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in phoronix_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "phoronix" {
		t.Errorf("Scheme = %q, want phoronix", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "phoronix" {
		t.Errorf("Identity.Binary = %q, want phoronix", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"wiki/Go", "page", "wiki/Go"},
		{"/about/", "page", "about"},
		{"https://" + Host + "/team/contact", "page", "team/contact"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("page", "wiki/Go")
	want := "https://" + Host + "/wiki/Go"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

// TestHostWiring mounts the driver in a kit Host (the runtime ant drives) and
// checks the round trip: a record mints to its URI, its body is readable, and a
// bare id resolves back to the same URI. The init in domain.go registers the
// domain, so kit.Open finds it.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Page{ID: "wiki/Go", URL: "https://" + Host + "/wiki/Go", Title: "Go", Body: "Go is a language."}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "phoronix://page/wiki/Go"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	if body, ok := h.Body(p); !ok || body == "" {
		t.Errorf("Body = (%q, %v), want non-empty", body, ok)
	}

	got, err := h.ResolveOn("phoronix", "about")
	if err != nil || got.String() != "phoronix://page/about" {
		t.Errorf("ResolveOn = (%q, %v), want phoronix://page/about", got.String(), err)
	}
}
