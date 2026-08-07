// Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License

package tabnasfeed_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tabnasfeed "github.com/tabnas/feed/go"
	jsonic "github.com/tabnas/jsonic/go"
)

// wellformedDir resolves the fetched kurtmckee/feedparser well-formed corpus.
// The corpus is NOT committed (scripts/fetch-feedparser.sh fetches it at a
// pinned commit into a gitignored dir), and its absence is a hard FAILURE —
// never a skip: a conformance test that quietly does not run is worse than
// no test at all.
func wellformedDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(requireCorpus(t, "feedparser", "fetch-feedparser.sh"), "wellformed")
}

// canon marshals a Go value via JSON and unmarshals it back to `any` so
// it has the same shape as readJSON output. This collapses pointers,
// custom struct tags, and integer-vs-float-64 differences.
func canon(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// buildParser returns a Jsonic instance with the Feed plugin in the
// requested format mode.
func buildParser(t *testing.T, format string) *jsonic.Jsonic {
	t.Helper()
	j := jsonic.Make()
	opts := map[string]any{}
	if format != "" {
		opts["format"] = format
	}
	if err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults, opts); err != nil {
		t.Fatalf("plugin init: %v", err)
	}
	return j
}

// The dialect/atom/native fixtures these three tests used to enumerate now
// live as TSV rows under test/spec, run by parity_test.go here and by
// ts/test/parity.test.ts — one mechanism instead of two. See test/AGENTS.md.

// --- feedparser well-formed corpus (fetched, see conformance_test.go) -----

func corpusFiles(t *testing.T, sub string) []string {
	t.Helper()
	dir := filepath.Join(wellformedDir(t), sub)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".xml") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// TestCorpusDetect and TestCorpusParseAtom used to live here. They are
// superseded by conformance_test.go, which runs the same corpus against a
// sound oracle: TestCorpusDetect expected every file under wellformed/atom/
// to be Atom 0.3 and every file under wellformed/rss/ to be RSS, but those
// upstream directories are not version-homogeneous (atom/entry_published_parsed.xml
// declares the Atom 1.0 namespace; rss/rss_version_090.xml is an RDF
// document), so that expectation was wrong, not merely strict. The targeted
// value checks below are kept.

// --- targeted value checks (from feedparser comments) ---------------------

func TestCorpusTargets(t *testing.T) {
	atomParse := buildParser(t, "atom")

	cases := []struct {
		path  string
		check func(t *testing.T, f tabnasfeed.AtomFeed)
	}{
		{"atom10/entry_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value != "Example Atom" {
				t.Fatalf("got %+v", f.Entries[0].Title)
			}
		}},
		{"atom10/entry_author_email.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Authors[0].Email != "me@example.com" {
				t.Fatalf("got %q", f.Entries[0].Authors[0].Email)
			}
		}},
		{"atom10/entry_author_name.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Authors[0].Name != "Example author" {
				t.Fatalf("got %q", f.Entries[0].Authors[0].Name)
			}
		}},
		{"atom10/entry_id.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].ID == "" {
				t.Fatalf("id missing")
			}
		}},
		{"atom10/entry_link_href.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if len(f.Entries[0].Links) == 0 || f.Entries[0].Links[0].Href == "" {
				t.Fatalf("link missing")
			}
		}},
		{"atom/entry_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value == "" {
				t.Fatalf("title missing")
			}
		}},
		{"atom/entry_issued.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Published == "" {
				t.Fatalf("published missing")
			}
		}},
		{"atom/entry_modified.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Updated == "" {
				t.Fatalf("updated missing")
			}
		}},
		{"rss/channel_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Title == nil || f.Title.Value != "Example feed" {
				t.Fatalf("got %+v", f.Title)
			}
		}},
		{"rss/item_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value != "Item 1 title" {
				t.Fatalf("got %+v", f.Entries[0].Title)
			}
		}},
		{"rss/item_link.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if len(f.Entries[0].Links) == 0 {
				t.Fatalf("links missing")
			}
		}},
		{"rss/item_guid.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].ID == "" {
				t.Fatalf("id missing")
			}
		}},
		{"rss/item_enclosure_url.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			var found bool
			for _, l := range f.Entries[0].Links {
				if l.Rel == "enclosure" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("enclosure link missing")
			}
		}},
		{"rdf/rdf_channel_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Title == nil || f.Title.Value != "Example feed" {
				t.Fatalf("got %+v", f.Title)
			}
		}},
		{"rdf/rdf_item_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value != "Example title" {
				t.Fatalf("got %+v", f.Entries[0].Title)
			}
		}},
		{"rdf/rss090_channel_title.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Title == nil || f.Title.Value != "Example title" {
				t.Fatalf("got %+v", f.Title)
			}
		}},
		{"rdf/rdf_item_rdf_about.xml", func(t *testing.T, f tabnasfeed.AtomFeed) {
			if f.Entries[0].ID != "http://example.org/1" {
				t.Fatalf("got %q", f.Entries[0].ID)
			}
		}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.path, func(t *testing.T) {
			full := filepath.Join(wellformedDir(t), c.path)
			src, err := os.ReadFile(full)
			if err != nil {
				t.Fatalf("read %s: %v", full, err)
			}
			got, err := atomParse.Parse(string(src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			af, ok := got.(tabnasfeed.AtomFeed)
			if !ok {
				t.Fatalf("expected AtomFeed, got %T", got)
			}
			c.check(t, af)
		})
	}
}

func TestErrorOnUnknownRoot(t *testing.T) {
	atomParse := buildParser(t, "atom")
	_, err := atomParse.Parse(`<not-a-feed/>`)
	if err == nil {
		t.Fatalf("expected error on unknown root")
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
