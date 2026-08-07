// Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License

package tabnasfeed_test

// Conformance against two third-party corpora. This file is the Go half of
// ts/test/feedvalidator.test.ts + ts/test/feedparser.test.ts + the shared
// helpers in ts/test/corpus.ts and ts/test/expect-eval.ts; the two runtimes
// classify and assert identically, so a divergence shows up as one going red.
//
//   rubys/feedvalidator      @ 2a8050b950594464b3923af249623b614774c138
//     -> scripts/fetch-feedvalidator.sh
//   kurtmckee/feedparser     @ a22c5521cbb109871f1a2318948581901bd47e26
//     -> scripts/fetch-feedparser.sh
//
// NEITHER corpus is committed to this repo. Both are fetched at a pinned
// commit into a gitignored directory. When a corpus is absent these tests
// FAIL LOUDLY — they never t.Skip(), because a conformance test that quietly
// does not run is worse than no test at all.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	tabnasfeed "github.com/tabnas/feed/go"
	jsonic "github.com/tabnas/jsonic/go"
	xml "github.com/tabnas/xml/go"
)

// --- corpus plumbing ------------------------------------------------------

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..")
}

// requireCorpus returns the corpus directory, fetching it first if it is not
// there. It NEVER skips: if the corpus cannot be made available the test
// fails, because a conformance test that quietly does not run is worse than
// no test at all. (`go test` has no pretest hook, which is exactly how the
// previous attempt ended up with a suite that silently never ran in CI.)
func requireCorpus(t *testing.T, dir, script string) string {
	t.Helper()
	root := repoRoot(t)
	full := filepath.Join(root, "test", dir)
	if _, err := os.Stat(full); err != nil {
		fetch := exec.Command("node", filepath.Join(root, "scripts", "fetch-corpus.mjs"), dir)
		fetch.Stdout, fetch.Stderr = os.Stdout, os.Stderr
		if ferr := fetch.Run(); ferr != nil {
			t.Logf("auto-fetch of the %s corpus failed: %v", dir, ferr)
		}
	}
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("\n\nCONFORMANCE CORPUS MISSING: %s\n"+
			"This suite cannot run without it, and must never be skipped.\n"+
			"Fetch it (pinned commit, idempotent):\n\n"+
			"    ./scripts/%s\n\n"+
			"See scripts/%s for the upstream URL and pinned SHA.\n", full, script, script)
	}
	return full
}

func walkXml(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".xml") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func relPath(t *testing.T, p string) string {
	r, err := filepath.Rel(repoRoot(t), p)
	if err != nil {
		return p
	}
	return r
}

var (
	reComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	rePI      = regexp.MustCompile(`(?s)<\?.*?\?>`)
	reDoctype = regexp.MustCompile(`(?is)<!DOCTYPE.*?>`)
	reRoot    = regexp.MustCompile(`<\s*([A-Za-z_][-A-Za-z0-9_:.]*)`)
	reSAX     = regexp.MustCompile(`Expect:[^\n]*SAXError`)
)

// rootLocalName reads the document element from the SOURCE TEXT, deliberately
// independent of our own parser, so classification cannot be biased by the
// thing under test.
func rootLocalName(src string) string {
	s := reComment.ReplaceAllString(src, "")
	s = rePI.ReplaceAllString(s, "")
	s = reDoctype.ReplaceAllString(s, "")
	m := reRoot.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	n := m[1]
	if i := strings.LastIndex(n, ":"); i >= 0 {
		n = n[i+1:]
	}
	return n
}

// The document elements @tabnas/feed claims: RSS 0.90-2.0, Atom 0.3/1.0.
func isFeedRoot(src string) bool {
	switch rootLocalName(src) {
	case "feed", "rss", "RDF":
		return true
	}
	return false
}

func fmtFails(fails []string) string {
	limit := 40
	var b strings.Builder
	for i, f := range fails {
		if i == limit {
			fmt.Fprintf(&b, "  ... and %d more\n", len(fails)-limit)
			break
		}
		fmt.Fprintf(&b, "  %s\n", f)
	}
	return b.String()
}

// atomParser builds the documented stack: jsonic + Feed, atom output.
func atomParser(t *testing.T) *jsonic.Jsonic {
	t.Helper()
	j := jsonic.Make()
	if err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults, map[string]any{"format": "atom"}); err != nil {
		t.Fatalf("plugin init: %v", err)
	}
	return j
}

func xmlParser(t *testing.T) *jsonic.Jsonic {
	t.Helper()
	j := jsonic.Make()
	if err := j.UseDefaults(xml.Xml, xml.Defaults); err != nil {
		t.Fatalf("xml plugin: %v", err)
	}
	return j
}

// safeParse turns a panic into an error, so a crash in the parser is recorded
// as a rejection rather than taking the whole suite down. The distinction is
// reported: a panic is a defect even when the classification "passes".
func safeParse(j *jsonic.Jsonic, src string) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %v", r)
		}
	}()
	return j.Parse(src)
}

// jsonShape round-trips a parse result to map[string]any so the value
// resolver below can be byte-for-byte the same logic as the TS one (the Go
// struct json tags are the TS property names).
func jsonShape(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// --- feedvalidator --------------------------------------------------------

func TestFeedValidatorConformance(t *testing.T) {
	suite := requireCorpus(t, "feedvalidator", "fetch-feedvalidator.sh")
	files := walkXml(t, filepath.Join(suite, "testcases"))
	if len(files) <= 1000 {
		t.Fatalf("feedvalidator corpus looks truncated: %d .xml files under %s; "+
			"re-run ./scripts/fetch-feedvalidator.sh", len(files), suite)
	}

	var sax, feeds, outOfClaim []string
	for _, p := range files {
		src := readFile(t, p)
		switch {
		case reSAX.MatchString(src):
			sax = append(sax, p)
		case isFeedRoot(src):
			feeds = append(feeds, p)
		default:
			outOfClaim = append(outOfClaim, p)
		}
	}

	j := atomParser(t)

	// 1. Upstream says NOT well-formed XML -> must be rejected.
	t.Run("invalid documents must be REJECTED", func(t *testing.T) {
		if len(sax) == 0 {
			t.Fatal("no SAXError cases found — corpus wrong?")
		}
		var fails []string
		for _, p := range sax {
			if _, err := safeParse(j, readFile(t, p)); err == nil {
				fails = append(fails, "ACCEPTED but upstream says not-well-formed: "+relPath(t, p))
			}
		}
		fmt.Printf("feedvalidator invalid: %d/%d rejected\n", len(sax)-len(fails), len(sax))
		if len(fails) > 0 {
			t.Fatalf("must-reject failures (%d/%d):\n%s", len(fails), len(sax), fmtFails(fails))
		}
	})

	// 2. Well-formed RSS/Atom -> must be accepted.
	t.Run("valid documents must be ACCEPTED", func(t *testing.T) {
		var fails []string
		for _, p := range feeds {
			got, err := safeParse(j, readFile(t, p))
			if err != nil {
				fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			af, ok := got.(tabnasfeed.AtomFeed)
			if !ok || af.Format != "atom" {
				fails = append(fails, fmt.Sprintf("%s: no atom-shaped result (%T)", relPath(t, p), got))
			}
		}
		roots := map[string]bool{}
		for _, p := range outOfClaim {
			roots[rootLocalName(readFile(t, p))] = true
		}
		var rs []string
		for r := range roots {
			rs = append(rs, r)
		}
		sort.Strings(rs)
		fmt.Printf("feedvalidator valid: %d/%d accepted (+%d documents outside the RSS/Atom claim, not asserted: %s)\n",
			len(feeds)-len(fails), len(feeds), len(outOfClaim), strings.Join(rs, ", "))
		if len(fails) > 0 {
			t.Fatalf("must-accept failures (%d/%d):\n%s", len(fails), len(feeds), fmtFails(fails))
		}
	})

	// 3. Accepting is only half a value check: the dialect must be right too.
	t.Run("detected dialect matches the corpus directory", func(t *testing.T) {
		jx := xmlParser(t)
		var fails []string
		checked := 0
		for _, p := range feeds {
			r := filepath.ToSlash(relPath(t, p))
			var want string
			switch {
			case strings.Contains(r, "/testcases/atom/"):
				want = "atom"
			case strings.Contains(r, "/testcases/rss20/"):
				want = "rss"
			default:
				continue
			}
			checked++
			root, err := safeParse(jx, readFile(t, p))
			if err != nil {
				fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			got := tabnasfeed.Detect(root)
			if got.Dialect != want {
				fails = append(fails, fmt.Sprintf("%s: dialect=%s/%s, want %s",
					relPath(t, p), got.Dialect, got.Version, want))
			}
		}
		fmt.Printf("feedvalidator detect: %d/%d correct dialect\n", checked-len(fails), checked)
		if len(fails) > 0 {
			t.Fatalf("dialect-detection failures (%d/%d):\n%s", len(fails), checked, fmtFails(fails))
		}
	})
}

// --- feedparser -----------------------------------------------------------

func TestFeedParserConformance(t *testing.T) {
	suite := requireCorpus(t, "feedparser", "fetch-feedparser.sh")
	wfAll := walkXml(t, filepath.Join(suite, "wellformed"))
	illAll := walkXml(t, filepath.Join(suite, "illformed"))
	if len(wfAll) <= 1000 {
		t.Fatalf("feedparser wellformed corpus looks truncated: %d files; "+
			"re-run ./scripts/fetch-feedparser.sh", len(wfAll))
	}
	if len(illAll) == 0 {
		t.Fatal("feedparser illformed corpus missing; re-run ./scripts/fetch-feedparser.sh")
	}

	var wf []string
	for _, p := range wfAll {
		if isFeedRoot(readFile(t, p)) {
			wf = append(wf, p)
		}
	}
	outOfClaim := len(wfAll) - len(wf)

	j := atomParser(t)

	t.Run("well-formed documents must PARSE", func(t *testing.T) {
		var fails []string
		for _, p := range wf {
			got, err := safeParse(j, readFile(t, p))
			if err != nil {
				fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			if af, ok := got.(tabnasfeed.AtomFeed); !ok || af.Format != "atom" {
				fails = append(fails, fmt.Sprintf("%s: not an atom-shaped result (%T)", relPath(t, p), got))
			}
		}
		fmt.Printf("feedparser wellformed: %d/%d parsed (+%d non-RSS/Atom roots, outside the claim, not asserted)\n",
			len(wf)-len(fails), len(wf), outOfClaim)
		if len(fails) > 0 {
			t.Fatalf("parse failures (%d/%d):\n%s", len(fails), len(wf), fmtFails(fails))
		}
	})

	t.Run("well-formed documents must produce the EXPECTED VALUE", func(t *testing.T) {
		var fails []string
		unmapped := map[string]int{}
		supported, checked, unmappedFiles := 0, 0, 0
		for _, p := range wf {
			src := readFile(t, p)
			clauses := parseExpect(src)
			if clauses == nil {
				continue
			}
			supported++
			got, err := safeParse(j, src)
			if err != nil {
				fails = append(fails, relPath(t, p)+": parse threw: "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			shape := jsonShape(got)
			sawUnmapped := false
			bad := ""
			for _, c := range clauses {
				v, why := resolveExpect(shape, c.steps)
				if why != "" {
					sawUnmapped = true
					unmapped[why]++
					continue
				}
				s, isStr := v.(string)
				if !isStr || s != c.want {
					bad = fmt.Sprintf("%s = %s, expected %q", c.path, jsonOf(v), c.want)
					break
				}
			}
			switch {
			case bad != "":
				fails = append(fails, relPath(t, p)+": "+bad)
			case sawUnmapped:
				unmappedFiles++
			default:
				checked++
			}
		}
		type kv struct {
			k string
			v int
		}
		var top []kv
		for k, v := range unmapped {
			top = append(top, kv{k, v})
		}
		sort.Slice(top, func(a, b int) bool {
			if top[a].v != top[b].v {
				return top[a].v > top[b].v
			}
			return top[a].k < top[b].k
		})
		if len(top) > 15 {
			top = top[:15]
		}
		var ts []string
		for _, e := range top {
			ts = append(ts, fmt.Sprintf("%s(%d)", e.k, e.v))
		}
		fmt.Printf("feedparser values: %d/%d correct (%d of %d files have a machine-checkable Expect; "+
			"%d use paths this harness does not map — known gap)\n  top unmapped paths: %s\n",
			checked, checked+len(fails), supported, len(wf), unmappedFiles, strings.Join(ts, " "))
		if len(fails) > 0 {
			t.Fatalf("value failures (%d/%d):\n%s", len(fails), checked+len(fails), fmtFails(fails))
		}
	})

	// NOTE (known gap, deliberately left RED rather than narrowed):
	// feedparser's `bozo` flag is broader than XML well-formedness — it also
	// covers declared-vs-actual character-encoding mismatches
	// (illformed/chardet/*) and GeoRSS/GML coordinate errors (illformed/geo/*),
	// neither of which a string-input XML parser can or should detect. Those
	// cases are left IN the corpus and left FAILING. Do not remove them to get
	// green; either the parser learns to reject them or the gap is stated in
	// writing here.
	t.Run("ill-formed documents must be REJECTED", func(t *testing.T) {
		var fails []string
		for _, p := range illAll {
			if _, err := safeParse(j, readFile(t, p)); err == nil {
				fails = append(fails, "ACCEPTED but upstream marks it ill-formed: "+relPath(t, p))
			}
		}
		fmt.Printf("feedparser illformed: %d/%d rejected\n", len(illAll)-len(fails), len(illAll))
		if len(fails) > 0 {
			t.Fatalf("must-reject failures (%d/%d):\n%s", len(fails), len(illAll), fmtFails(fails))
		}
	})

	// Dialect / version detection. Two oracles, both the corpus's own:
	//  * `wellformed/atom10/` and `wellformed/rdf/` are version-homogeneous
	//    upstream directories. `wellformed/atom/` and `wellformed/rss/` are
	//    NOT (atom/entry_published_parsed.xml declares the Atom 1.0 namespace;
	//    rss/rss_version_090.xml is an RDF document), so no directory-level
	//    version expectation is asserted for those two.
	//  * The upstream `Expect: ... version == 'X'` annotation.
	t.Run("dialect/version detection", func(t *testing.T) {
		jx := xmlParser(t)
		groups := []struct {
			sub      string
			dialect  string
			versions []string
		}{
			{"atom10", "atom", []string{"atom10"}},
			{"rdf", "rdf", []string{"rss10", "rss090"}},
		}
		for _, g := range groups {
			g := g
			t.Run(g.sub, func(t *testing.T) {
				files := walkXml(t, filepath.Join(suite, "wellformed", g.sub))
				if len(files) == 0 {
					t.Fatalf("wellformed/%s is empty — corpus wrong?", g.sub)
				}
				var fails []string
				for _, p := range files {
					root, err := safeParse(jx, readFile(t, p))
					if err != nil {
						fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
						continue
					}
					got := tabnasfeed.Detect(root)
					if got.Dialect != g.dialect || !contains(g.versions, got.Version) {
						fails = append(fails, fmt.Sprintf("%s: %s/%s", relPath(t, p), got.Dialect, got.Version))
					}
				}
				if len(fails) > 0 {
					t.Fatalf("%s detection failures (%d/%d):\n%s", g.sub, len(fails), len(files), fmtFails(fails))
				}
			})
		}

		t.Run("dialect matches the document element", func(t *testing.T) {
			want := map[string]string{"feed": "atom", "rss": "rss", "RDF": "rdf"}
			var fails []string
			for _, p := range wf {
				src := readFile(t, p)
				root, err := safeParse(jx, src)
				if err != nil {
					fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
					continue
				}
				got := tabnasfeed.Detect(root)
				if got.Dialect != want[rootLocalName(src)] {
					fails = append(fails, fmt.Sprintf("%s: %s (root <%s>)", relPath(t, p), got.Dialect, rootLocalName(src)))
				}
			}
			fmt.Printf("feedparser dialect: %d/%d correct\n", len(wf)-len(fails), len(wf))
			if len(fails) > 0 {
				t.Fatalf("dialect failures (%d/%d):\n%s", len(fails), len(wf), fmtFails(fails))
			}
		})

		t.Run("upstream version annotations hold", func(t *testing.T) {
			reVer := regexp.MustCompile(`version == '([a-z0-9]+)'`)
			var fails []string
			checked := 0
			for _, p := range wf {
				src := readFile(t, p)
				m := reVer.FindStringSubmatch(src)
				if m == nil {
					continue
				}
				checked++
				root, err := safeParse(jx, src)
				if err != nil {
					fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
					continue
				}
				if got := tabnasfeed.Detect(root); got.Version != m[1] {
					fails = append(fails, fmt.Sprintf("%s: %s, upstream says %s", relPath(t, p), got.Version, m[1]))
				}
			}
			if checked == 0 {
				t.Fatal("no version annotations found — corpus wrong?")
			}
			fmt.Printf("feedparser version: %d/%d correct\n", checked-len(fails), checked)
			if len(fails) > 0 {
				t.Fatalf("version failures (%d/%d):\n%s", len(fails), checked, fmtFails(fails))
			}
		})
	})
}

func jsonOf(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<unmarshalable>"
	}
	return string(b)
}

// --- feedparser `Expect:` evaluator ---------------------------------------
//
// Mirror of ts/test/expect-eval.ts — see that file for the full rationale.
// Supported form: `not bozo and <path> == '<string>'`, any number of
// `and`-joined clauses. Everything else (time tuples, len(), has_key(), dict
// literals, bare truthiness) is unsupported and counted, never silently
// passed.

type expectClause struct {
	path  string
	steps []any // string key or int index
	want  string
}

var (
	reExpect = regexp.MustCompile(`Expect:[ \t]*(.*)`)
	reClause = regexp.MustCompile(`^((?:feed|entries\[\d+\])(?:\[(?:'[^']*'|\d+)\])*)\s*==\s*'((?:[^'\\]|\\.)*)'$`)
	reHead   = regexp.MustCompile(`^(?:feed|entries\[(\d+)\])`)
	reStep   = regexp.MustCompile(`\[(?:'([^']*)'|(\d+))\]`)
	reParen  = regexp.MustCompile(`^\((.*)\)$`)
	reAnd    = regexp.MustCompile(`\s+and\s+`)
)

func parseExpect(src string) []expectClause {
	m := reExpect.FindStringSubmatch(src)
	if m == nil {
		return nil
	}
	e := strings.TrimSpace(m[1])
	const pre = "not bozo and "
	if !strings.HasPrefix(e, pre) {
		return nil
	}
	e = strings.TrimSpace(e[len(pre):])

	var out []expectClause
	for _, part := range reAnd.Split(e, -1) {
		part = strings.TrimSpace(part)
		if p := reParen.FindStringSubmatch(part); p != nil {
			part = strings.TrimSpace(p[1])
		}
		cm := reClause.FindStringSubmatch(part)
		if cm == nil {
			return nil
		}
		var steps []any
		head := reHead.FindStringSubmatch(cm[1])
		if head[1] == "" {
			steps = append(steps, "feed")
		} else {
			n, _ := strconv.Atoi(head[1])
			steps = append(steps, "entries", n)
		}
		for _, s := range reStep.FindAllStringSubmatch(cm[1][len(head[0]):], -1) {
			if s[2] != "" {
				n, _ := strconv.Atoi(s[2])
				steps = append(steps, n)
			} else {
				steps = append(steps, s[1])
			}
		}
		want := cm[2]
		want = strings.ReplaceAll(want, `\'`, `'`)
		want = strings.ReplaceAll(want, `\"`, `"`)
		want = strings.ReplaceAll(want, `\\`, `\`)
		out = append(out, expectClause{path: cm[1], steps: steps, want: want})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var (
	feedText = map[string]string{
		"title": "title", "subtitle": "subtitle", "tagline": "subtitle",
		"description": "subtitle", "info": "subtitle",
		"rights": "rights", "copyright": "rights",
	}
	entryText = map[string]string{
		"title": "title", "summary": "summary", "description": "summary",
		"rights": "rights", "copyright": "rights",
	}
	personKey = map[string]string{
		"name": "name", "email": "email", "href": "uri", "url": "uri", "uri": "uri",
	}
	// feedparser reports text-construct types as MIME types; the Atom shape
	// keeps RFC 4287's text/html/xhtml tokens.
	mimeOf = map[string]string{
		"text": "text/plain", "html": "text/html", "xhtml": "application/xhtml+xml",
	}
)

func mget(v any, k string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m[k]
}

func aget(v any, i int) any {
	a, ok := v.([]any)
	if !ok || i < 0 || i >= len(a) {
		return nil
	}
	return a[i]
}

func firstAlternate(links any) any {
	a, ok := links.([]any)
	if !ok {
		return nil
	}
	for _, l := range a {
		rel, _ := mget(l, "rel").(string)
		if rel == "" || rel == "alternate" {
			return mget(l, "href")
		}
	}
	return nil
}

func textAt(obj any, prop string, tail []any) (any, string) {
	tv := mget(obj, prop)
	if len(tail) == 0 {
		return mget(tv, "value"), ""
	}
	if len(tail) == 1 {
		if s, ok := tail[0].(string); ok {
			if s == "value" {
				return mget(tv, "value"), ""
			}
			if s == "type" {
				ty, _ := mget(tv, "type").(string)
				if m, ok := mimeOf[ty]; ok {
					return m, ""
				}
				return nil, ""
			}
		}
	}
	return nil, "detail." + joinSteps(tail)
}

func joinSteps(s []any) string {
	var parts []string
	for _, x := range s {
		parts = append(parts, fmt.Sprint(x))
	}
	return strings.Join(parts, ".")
}

func has(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// resolveExpect returns (value, "") on success, or (nil, why) when the path
// has no mapping onto the Atom shape.
func resolveExpect(feed any, steps []any) (any, string) {
	var obj any
	var path []any
	isEntry := steps[0] == "entries"
	if isEntry {
		es, ok := mget(feed, "entries").([]any)
		idx := steps[1].(int)
		if !ok || idx >= len(es) {
			return nil, "no such entry"
		}
		obj = es[idx]
		path = steps[2:]
	} else {
		obj = feed
		path = steps[1:]
	}
	if len(path) == 0 {
		return nil, "whole-object comparison"
	}

	k, isStr := path[0].(string)
	tail := path[1:]
	if !isStr {
		return nil, "index-at-root"
	}

	text := feedText
	if isEntry {
		text = entryText
	}
	if prop, ok := text[k]; ok {
		return textAt(obj, prop, tail)
	}
	if strings.HasSuffix(k, "_detail") {
		if prop, ok := text[strings.TrimSuffix(k, "_detail")]; ok {
			return textAt(obj, prop, tail)
		}
	}

	n := func(i int) (int, bool) {
		v, ok := tail[i].(int)
		return v, ok
	}
	s := func(i int) string {
		v, _ := tail[i].(string)
		return v
	}

	switch {
	case (k == "id" || k == "guid") && len(tail) == 0:
		return mget(obj, "id"), ""
	case k == "updated" && len(tail) == 0:
		return mget(obj, "updated"), ""
	case k == "published" && len(tail) == 0:
		return mget(obj, "published"), ""
	case k == "link" && len(tail) == 0:
		return firstAlternate(mget(obj, "links")), ""
	case k == "links" && len(tail) == 2:
		if i, ok := n(0); ok && has([]string{"href", "rel", "type", "title"}, s(1)) {
			return mget(aget(mget(obj, "links"), i), s(1)), ""
		}
	case k == "author_detail" && len(tail) == 1:
		if pk, ok := personKey[s(0)]; ok {
			return mget(aget(mget(obj, "authors"), 0), pk), ""
		}
	case (k == "authors" || k == "contributors") && len(tail) == 2:
		if i, ok := n(0); ok {
			if pk, ok2 := personKey[s(1)]; ok2 {
				return mget(aget(mget(obj, k), i), pk), ""
			}
		}
	case k == "tags" && len(tail) == 2:
		if i, ok := n(0); ok && has([]string{"term", "scheme", "label"}, s(1)) {
			return mget(aget(mget(obj, "categories"), i), s(1)), ""
		}
	case k == "generator" && len(tail) == 0:
		return mget(mget(obj, "generator"), "value"), ""
	case k == "content" && len(tail) == 2:
		if i, ok := n(0); ok && has([]string{"value", "type"}, s(1)) {
			if i != 0 {
				return nil, "content[n>0]"
			}
			c := mget(obj, "content")
			if s(1) == "type" {
				ty, _ := mget(c, "type").(string)
				if m, ok := mimeOf[ty]; ok {
					return m, ""
				}
				return mget(c, "type"), ""
			}
			return mget(c, "value"), ""
		}
	case k == "image" && len(tail) == 1 && s(0) == "href":
		return mget(obj, "logo"), ""
	case k == "source" && len(tail) > 0:
		src := mget(obj, "source")
		if src == nil {
			return nil, ""
		}
		return resolveExpect(src, append([]any{"feed"}, tail...))
	}

	prefix := "feed."
	if isEntry {
		prefix = "entry."
	}
	return nil, prefix + joinSteps(path)
}
