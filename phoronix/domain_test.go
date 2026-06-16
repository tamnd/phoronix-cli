package phoronix

import (
	"testing"
)

// domain_test.go tests the pure, network-free parts of the domain: Info fields,
// static data, and helper functions. The HTTP behaviour is covered in
// phoronix_test.go (external test package, uses httptest).

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

func TestCategories(t *testing.T) {
	if len(Categories) == 0 {
		t.Fatal("Categories is empty")
	}
	found := false
	for _, c := range Categories {
		if c == "linux" {
			found = true
		}
	}
	if !found {
		t.Error("Categories does not include 'linux'")
	}
}

func TestStripHTMLTags(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"&amp; &lt; &gt;", "& < >"},
		{"  spaces   inside  ", "spaces inside"},
	}
	for _, tc := range cases {
		got := stripHTMLTags(tc.in)
		if got != tc.want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://www.phoronix.com/news/Linux-613-AMD-Power", "Linux-613-AMD-Power"},
		{"https://www.phoronix.com/news/Mesa-25-0-RC1", "Mesa-25-0-RC1"},
	}
	for _, tc := range cases {
		got := slugFromURL(tc.in)
		if got != tc.want {
			t.Errorf("slugFromURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCleanAuthor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Michael Larabel", "Michael Larabel"},
		{"Michael Larabel <m@phoronix.com>", "Michael Larabel"},
		{"  ", ""},
		{"editor@phoronix.com", ""},
	}
	for _, tc := range cases {
		got := cleanAuthor(tc.in)
		if got != tc.want {
			t.Errorf("cleanAuthor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsBlocked(t *testing.T) {
	blocked := [][]byte{
		[]byte("<title>Just a moment...</title>"),
		[]byte("Enable JavaScript and cookies to continue"),
		[]byte("cf-browser-verification"),
	}
	for _, b := range blocked {
		if !isBlocked(b) {
			t.Errorf("isBlocked(%q) should be true", string(b))
		}
	}
	notBlocked := []byte("<html><body>Normal page content</body></html>")
	if isBlocked(notBlocked) {
		t.Error("isBlocked(normal page) should be false")
	}
}
