// Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License

package feed_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	jsonic "github.com/tabnas/jsonic/go"
	feed "github.com/tabnas/feed/go"
	xml "github.com/tabnas/xml/go"
)

// specsDir resolves test/specs from the package directory.
func specsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// go/ -> repo root
	return filepath.Join(wd, "..", "test", "specs")
}

// wellformedDir resolves test/feedparser-wellformed.
func wellformedDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "test", "feedparser-wellformed")
}

// readJSON loads and parses a JSON file into an `any`. Used to canonicalise
// expected and actual structures so deep-equal compares them by shape, not
// by Go-typed-vs-decoded details.
func readJSON(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return v
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
	if err := j.UseDefaults(feed.Feed, feed.Defaults, opts); err != nil {
		t.Fatalf("plugin init: %v", err)
	}
	return j
}

// listSpecs returns base names (without extension) of all .xml fixtures
// in test/specs/.
func listSpecs(t *testing.T) []string {
	t.Helper()
	dir := specsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read specs dir %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".xml") {
			names = append(names, strings.TrimSuffix(n, ".xml"))
		}
	}
	sort.Strings(names)
	return names
}


func TestSpecsDetect(t *testing.T) {
	names := listSpecs(t)
	if len(names) == 0 {
		t.Fatalf("no specs found")
	}
	rawParse := buildParser(t, "raw")
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			expectPath := filepath.Join(specsDir(t), name+".detect.json")
			if _, err := os.Stat(expectPath); err != nil {
				t.Skipf("no .detect.json for %s", name)
			}
			src, err := os.ReadFile(filepath.Join(specsDir(t), name+".xml"))
			if err != nil {
				t.Fatalf("read xml: %v", err)
			}
			root, err := rawParse.Parse(string(src))
			if err != nil {
				t.Fatalf("parse xml: %v", err)
			}
			got := feed.Detect(root)
			want := readJSON(t, expectPath)
			gotJSON := canon(t, got)
			if !reflect.DeepEqual(gotJSON, want) {
				t.Fatalf("detect mismatch:\n  got:  %v\n  want: %v", gotJSON, want)
			}
		})
	}
}

func TestSpecsAtom(t *testing.T) {
	names := listSpecs(t)
	atomParse := buildParser(t, "atom")
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			expectPath := filepath.Join(specsDir(t), name+".atom.json")
			if _, err := os.Stat(expectPath); err != nil {
				t.Skipf("no .atom.json for %s", name)
			}
			src, err := os.ReadFile(filepath.Join(specsDir(t), name+".xml"))
			if err != nil {
				t.Fatalf("read xml: %v", err)
			}
			got, err := atomParse.Parse(string(src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			gotJSON := canon(t, got)
			want := readJSON(t, expectPath)
			if !reflect.DeepEqual(gotJSON, want) {
				gj, _ := json.MarshalIndent(gotJSON, "", "  ")
				wj, _ := json.MarshalIndent(want, "", "  ")
				t.Fatalf("atom mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, gj, wj)
			}
		})
	}
}

func TestSpecsNative(t *testing.T) {
	names := listSpecs(t)
	nativeParse := buildParser(t, "native")
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			expectPath := filepath.Join(specsDir(t), name+".native.json")
			if _, err := os.Stat(expectPath); err != nil {
				t.Skipf("no .native.json for %s", name)
			}
			src, err := os.ReadFile(filepath.Join(specsDir(t), name+".xml"))
			if err != nil {
				t.Fatalf("read xml: %v", err)
			}
			got, err := nativeParse.Parse(string(src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			gotJSON := canon(t, got)
			want := readJSON(t, expectPath)
			if !reflect.DeepEqual(gotJSON, want) {
				gj, _ := json.MarshalIndent(gotJSON, "", "  ")
				wj, _ := json.MarshalIndent(want, "", "  ")
				t.Fatalf("native mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, gj, wj)
			}
		})
	}
}


// --- feedparser-wellformed corpus -----------------------------------------

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

type expected struct {
	dialect  string
	versions []string
}

var corpusExpect = map[string]expected{
	"atom10": {dialect: "atom", versions: []string{"atom10"}},
	"atom":   {dialect: "atom", versions: []string{"atom03"}},
	"rss":    {dialect: "rss", versions: []string{"rss20", "rss092", "rss091u", "rss091n"}},
	"rdf":    {dialect: "rdf", versions: []string{"rss10", "rss090"}},
}

func TestCorpusDetect(t *testing.T) {
	if _, err := os.Stat(wellformedDir(t)); err != nil {
		t.Skip("feedparser-wellformed not vendored")
	}
	// Build a raw-format parser by registering the xml plugin directly;
	// we don't need feed conversion to call Detect.
	j := jsonic.Make()
	if err := j.UseDefaults(xml.Xml, xml.Defaults); err != nil {
		t.Fatalf("xml plugin: %v", err)
	}

	for sub, expect := range corpusExpect {
		sub, expect := sub, expect
		t.Run(sub, func(t *testing.T) {
			var fails []string
			for _, path := range corpusFiles(t, sub) {
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				root, err := j.Parse(string(src))
				if err != nil {
					fails = append(fails, fmt.Sprintf("%s: parse err %v", filepath.Base(path), err))
					continue
				}
				got := feed.Detect(root)
				if got.Dialect != expect.dialect {
					fails = append(fails, fmt.Sprintf("%s: dialect=%s", filepath.Base(path), got.Dialect))
					continue
				}
				if !contains(expect.versions, got.Version) {
					fails = append(fails, fmt.Sprintf("%s: version=%s", filepath.Base(path), got.Version))
				}
			}
			if len(fails) > 0 {
				t.Fatalf("%s detection failures:\n  %s", sub, strings.Join(fails, "\n  "))
			}
		})
	}
}

func TestCorpusParseAtom(t *testing.T) {
	if _, err := os.Stat(wellformedDir(t)); err != nil {
		t.Skip("feedparser-wellformed not vendored")
	}
	atomParse := buildParser(t, "atom")
	for sub := range corpusExpect {
		sub := sub
		t.Run(sub, func(t *testing.T) {
			var fails []string
			for _, path := range corpusFiles(t, sub) {
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				got, err := atomParse.Parse(string(src))
				if err != nil {
					fails = append(fails, fmt.Sprintf("%s: %v", filepath.Base(path), err))
					continue
				}
				if af, ok := got.(feed.AtomFeed); !ok || af.Format != "atom" {
					fails = append(fails, fmt.Sprintf("%s: bad shape %T", filepath.Base(path), got))
				}
			}
			if len(fails) > 0 {
				t.Fatalf("%s parse failures:\n  %s", sub, strings.Join(fails, "\n  "))
			}
		})
	}
}


// --- targeted value checks (from feedparser comments) ---------------------

func TestCorpusTargets(t *testing.T) {
	if _, err := os.Stat(wellformedDir(t)); err != nil {
		t.Skip("feedparser-wellformed not vendored")
	}
	atomParse := buildParser(t, "atom")

	cases := []struct {
		path  string
		check func(t *testing.T, f feed.AtomFeed)
	}{
		{"atom10/entry_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value != "Example Atom" {
				t.Fatalf("got %+v", f.Entries[0].Title)
			}
		}},
		{"atom10/entry_author_email.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Authors[0].Email != "me@example.com" {
				t.Fatalf("got %q", f.Entries[0].Authors[0].Email)
			}
		}},
		{"atom10/entry_author_name.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Authors[0].Name != "Example author" {
				t.Fatalf("got %q", f.Entries[0].Authors[0].Name)
			}
		}},
		{"atom10/entry_id.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].ID == "" {
				t.Fatalf("id missing")
			}
		}},
		{"atom10/entry_link_href.xml", func(t *testing.T, f feed.AtomFeed) {
			if len(f.Entries[0].Links) == 0 || f.Entries[0].Links[0].Href == "" {
				t.Fatalf("link missing")
			}
		}},
		{"atom/entry_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value == "" {
				t.Fatalf("title missing")
			}
		}},
		{"atom/entry_issued.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Published == "" {
				t.Fatalf("published missing")
			}
		}},
		{"atom/entry_modified.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Updated == "" {
				t.Fatalf("updated missing")
			}
		}},
		{"rss/channel_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Title == nil || f.Title.Value != "Example feed" {
				t.Fatalf("got %+v", f.Title)
			}
		}},
		{"rss/item_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value != "Item 1 title" {
				t.Fatalf("got %+v", f.Entries[0].Title)
			}
		}},
		{"rss/item_link.xml", func(t *testing.T, f feed.AtomFeed) {
			if len(f.Entries[0].Links) == 0 {
				t.Fatalf("links missing")
			}
		}},
		{"rss/item_guid.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].ID == "" {
				t.Fatalf("id missing")
			}
		}},
		{"rss/item_enclosure_url.xml", func(t *testing.T, f feed.AtomFeed) {
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
		{"rdf/rdf_channel_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Title == nil || f.Title.Value != "Example feed" {
				t.Fatalf("got %+v", f.Title)
			}
		}},
		{"rdf/rdf_item_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Entries[0].Title == nil || f.Entries[0].Title.Value != "Example title" {
				t.Fatalf("got %+v", f.Entries[0].Title)
			}
		}},
		{"rdf/rss090_channel_title.xml", func(t *testing.T, f feed.AtomFeed) {
			if f.Title == nil || f.Title.Value != "Example title" {
				t.Fatalf("got %+v", f.Title)
			}
		}},
		{"rdf/rdf_item_rdf_about.xml", func(t *testing.T, f feed.AtomFeed) {
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
			af, ok := got.(feed.AtomFeed)
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
