// Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License

package tabnasfeed_test

// Conformance against the rubys/feedvalidator corpus — the test suite behind
// the W3C Feed Validation Service, and the most widely cited third-party
// corpus for RSS/Atom.
//
//	upstream: https://github.com/rubys/feedvalidator
//	pinned:   2a8050b950594464b3923af249623b614774c138
//	fetch:    ./scripts/fetch-feedvalidator.sh
//
// This file is the Go half of ts/test/feedvalidator.test.ts; the two runtimes
// classify and assert identically, so a divergence shows up as one going red.
//
// The corpus is NOT committed to this repo. It is fetched at a pinned commit
// into a gitignored directory. When it is absent these tests FAIL LOUDLY —
// they never t.Skip(), because a conformance test that quietly does not run
// is worse than no test at all.

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
)

// --- corpus plumbing ------------------------------------------------------

func conformRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..")
}

// requireCorpus returns the corpus directory, fetching it first if it is not
// there. It NEVER skips: if the corpus cannot be made available the test
// fails. (`go test` has no pretest hook, which is exactly how a previous
// attempt ended up with a suite that silently never ran in CI.)
func requireCorpus(t *testing.T, dir, script string) string {
	t.Helper()
	root := conformRepoRoot(t)
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
	r, err := filepath.Rel(conformRepoRoot(t), p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
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

// conformParser builds the documented stack: jsonic + Feed.
func conformParser(t *testing.T, format string) *jsonic.Jsonic {
	t.Helper()
	j := jsonic.Make()
	if err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults,
		map[string]any{"format": format}); err != nil {
		t.Fatalf("plugin init: %v", err)
	}
	return j
}

// safeParse turns a panic into an error, so a crash in the parser is recorded
// as a rejection rather than taking the whole suite down.
func safeParse(j *jsonic.Jsonic, src string) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %v", r)
		}
	}()
	return j.Parse(src)
}

// --- feedvalidator --------------------------------------------------------

// notWellFormed: four corpus documents that are objectively NOT well-formed
// but carry an `Expect:` naming a validator-level diagnostic instead of
// `SAXError` — upstream annotates the thing the case is *about*, and a real
// SAX parse of any of these fails first. They are moved into the must-REJECT
// bucket, not excused: the violation is quoted from the file, and the set is
// asserted to be exactly this list, so a fifth one cannot be added silently
// and a fixed one cannot stay listed. Kept byte-identical to the
// NOT_WELL_FORMED map in ts/test/feedvalidator.test.ts.
var notWellFormed = map[string]string{
	// <copyright type="application/xhtml+xml"> ... </rights>
	"atom/must/feed_copyright_is_inline.xml": `XML 1.0 3 WFC "Element Type Match": <copyright> is closed by </rights>`,
	// <copyright type="text/html" mode="xml"> ... </rights>
	"atom/must/feed_copyright_is_inline_2.xml": `XML 1.0 3 WFC "Element Type Match": <copyright> is closed by </rights>`,
	// <sx:sharing .../> is self-closing, then a stray </sx:sharing> follows,
	// so the end tag lands on the still-open <feed>.
	"ext/feedsync/sharing_until_rfc822.xml": `XML 1.0 3 WFC "Element Type Match": stray </sx:sharing> after a self-closed <sx:sharing/>`,
	// <invalid:tag xmlns:bogus="tag:foo.bar"/> — declares `bogus`, uses
	// `invalid`. The Expect is about the bogus namespace URI, but the
	// undeclared prefix is a hard error first.
	"atom/6.1/invalid-namespace.xml": `Namespaces in XML 1.0 NSC "Prefix Declared": prefix ` + "`invalid`" + ` is never declared`,
}

const testcasesPrefix = "test/feedvalidator/testcases/"

func TestFeedValidatorConformance(t *testing.T) {
	suite := requireCorpus(t, "feedvalidator", "fetch-feedvalidator.sh")
	files := walkXml(t, filepath.Join(suite, "testcases"))
	if len(files) <= 1000 {
		t.Fatalf("feedvalidator corpus looks truncated: %d .xml files under %s; "+
			"re-run ./scripts/fetch-feedvalidator.sh", len(files), suite)
	}

	var sax, feeds, outOfClaim []string
	seenNwf := map[string]bool{}
	for _, p := range files {
		src := readFile(t, p)
		key := strings.TrimPrefix(relPath(t, p), testcasesPrefix)
		switch {
		case notWellFormed[key] != "":
			seenNwf[key] = true
			sax = append(sax, p)
		case reSAX.MatchString(src):
			sax = append(sax, p)
		case isFeedRoot(src):
			feeds = append(feeds, p)
		default:
			outOfClaim = append(outOfClaim, p)
		}
	}

	// Guard the reclassification: every listed path must exist in the corpus,
	// so a rename upstream turns into a red test, not a silent exemption.
	if len(seenNwf) != len(notWellFormed) {
		var missing []string
		for k := range notWellFormed {
			if !seenNwf[k] {
				missing = append(missing, k)
			}
		}
		sort.Strings(missing)
		t.Fatalf("notWellFormed lists paths that are not in the fetched corpus: %v", missing)
	}

	j := conformParser(t, "atom")

	// 1. Not well-formed -> must be rejected.
	t.Run("invalid documents must be REJECTED", func(t *testing.T) {
		if len(sax) == 0 {
			t.Fatal("no SAXError cases found — corpus wrong?")
		}
		var fails []string
		for _, p := range sax {
			if _, err := safeParse(j, readFile(t, p)); err == nil {
				fails = append(fails, "ACCEPTED but not well-formed: "+relPath(t, p))
			}
		}
		fmt.Printf("feedvalidator invalid: %d/%d rejected (%d annotated \"Expect: SAXError\" + %d reclassified)\n",
			len(sax)-len(fails), len(sax), len(sax)-len(seenNwf), len(seenNwf))
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
		jraw := conformParser(t, "raw")
		var fails []string
		checked := 0
		for _, p := range feeds {
			r := relPath(t, p)
			var want string
			switch {
			case strings.HasPrefix(r, testcasesPrefix+"atom/"):
				want = "atom"
			case strings.HasPrefix(r, testcasesPrefix+"rss20/"):
				want = "rss"
			default:
				continue
			}
			checked++
			root, err := safeParse(jraw, readFile(t, p))
			if err != nil {
				fails = append(fails, r+": "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			got := tabnasfeed.Detect(root)
			if got.Dialect != want {
				fails = append(fails, fmt.Sprintf("%s: dialect=%s/%s, want %s",
					r, got.Dialect, got.Version, want))
			}
		}
		fmt.Printf("feedvalidator detect: %d/%d correct dialect\n", checked-len(fails), checked)
		// fails is also empty when nothing was checked. The want is keyed off
		// the corpus path, so a path-shape change would zero checked and pass
		// this test while asserting nothing. Pin the floor.
		if checked <= 1000 {
			t.Fatalf("only %d documents classified by directory", checked)
		}
		if len(fails) > 0 {
			t.Fatalf("dialect-detection failures (%d/%d):\n%s", len(fails), checked, fmtFails(fails))
		}
	})
}

// --- feedparser -----------------------------------------------------------
//
// Twin of ts/test/feedparser-conformance.test.ts. `scripts/fetch-corpus.mjs`
// has been fetching test/feedparser/ for both runtimes since the corpus
// landed, but nothing read it. This is the Go consumer.
//
// The two enumerated sets below (feedParserAcceptedIllformed and
// feedParserVersionKnownWrong) and the two value floors are kept
// byte-for-byte identical to the TypeScript twin, so a runtime divergence
// shows up as one side going red rather than as two drifting baselines.

// Measured on main at 7d2103f (2026-08-09): 375 of 1360 machine-checkable
// upstream `Expect:` assertions hold. This is a RATCHET: raise both numbers
// when the parser improves, never lower either to get green. The denominator
// is floored too, so dropping value checks cannot be used to improve the
// ratio.
const (
	feedParserValueCorrectFloor = 375
	feedParserValueCheckedFloor = 1360
)

// Upstream's illformed/ directory is keyed off feedparser's `bozo` flag,
// which is much broader than XML well-formedness. These documents are
// ACCEPTED by @tabnas/feed and each is listed with the reason, so the set is
// a statement rather than an excuse. It is asserted to be EXACTLY this list:
// a newly-accepted ill-formed document is red, and so is a listed document
// that starts being rejected (its entry must then be deleted).
var feedParserAcceptedIllformed = map[string]string{
	// Well-formed XML carrying an invalid DOCTYPE. Upstream itself annotates
	// this `Expect: not bozo and feed['title'] == 'found'`, so accepting it
	// is the CORRECT behaviour; it sits in illformed/ for a different reason.
	"always_strip_doctype.xml": "well-formed; upstream Expect is `not bozo`, so accepting is correct",

	// Declared-vs-actual character encoding mismatches. Detecting these needs
	// the raw byte stream and a charset detector; @tabnas/feed is handed an
	// already-decoded string, so the evidence is gone before it is called.
	"chardet/big5.xml":        "encoding detection: needs raw bytes, not a decoded string",
	"chardet/eucjp.xml":       "encoding detection: needs raw bytes, not a decoded string",
	"chardet/euckr.xml":       "encoding detection: needs raw bytes, not a decoded string",
	"chardet/gb2312.xml":      "encoding detection: needs raw bytes, not a decoded string",
	"chardet/koi8r.xml":       "encoding detection: needs raw bytes, not a decoded string",
	"chardet/shiftjis.xml":    "encoding detection: needs raw bytes, not a decoded string",
	"chardet/tis620.xml":      "encoding detection: needs raw bytes, not a decoded string",
	"chardet/windows1255.xml": "encoding detection: needs raw bytes, not a decoded string",

	// GeoRSS / GML coordinate errors. Well-formed XML; the defect is in the
	// meaning of an extension element @tabnas/feed does not model.
	"geo/georss_point_no_coords.xml":             "GeoRSS semantics, not XML well-formedness",
	"geo/georss_polygon_insufficient_coords.xml": "GeoRSS semantics, not XML well-formedness",
	"geo/gml_point.xml":                          "GML semantics, not XML well-formedness",

	// Well-formed iso-8859-7 document; upstream records that the non-ASCII
	// date crashed its own date parser. @tabnas/feed does not parse dates.
	"http_high_bit_date.xml": "upstream records a date-parser crash, not a well-formedness defect",
}

// Five documents where Detect() disagrees with the upstream annotation, each
// recorded with what we currently report — exact in both directions.
var feedParserVersionKnownWrong = map[string]string{
	// RSS 0.90 is an RDF document; Detect reports the RDF-era RSS 1.0.
	"rss/rss_version_090.xml": "rss10",
	// Netscape vs Userland 0.91 are told apart by the DOCTYPE, which Detect
	// does not read.
	"rss/rss_version_091_netscape.xml": "rss091u",
	// 0.93 / 0.94 are not modelled; both collapse onto 0.92.
	"rss/rss_version_093.xml": "rss092",
	"rss/rss_version_094.xml": "rss092",
	// <rss> with no version attribute: upstream reports the bare 'rss'.
	"rss/rss_version_missing.xml": "rss20",
}

const (
	feedParserWfPrefix  = "test/feedparser/wellformed/"
	feedParserIllPrefix = "test/feedparser/illformed/"
)

func TestFeedParserConformance(t *testing.T) {
	suite := requireCorpus(t, "feedparser", "fetch-feedparser.sh")
	wfAll := walkXml(t, filepath.Join(suite, "wellformed"))
	illAll := walkXml(t, filepath.Join(suite, "illformed"))
	// Floors on the corpus itself: a truncated fetch would otherwise shrink
	// every denominator below and turn this file green while measuring
	// almost nothing.
	if len(wfAll) <= 1000 {
		t.Fatalf("feedparser wellformed corpus looks truncated: %d files; "+
			"re-run ./scripts/fetch-feedparser.sh", len(wfAll))
	}
	if len(illAll) == 0 {
		t.Fatal("feedparser illformed corpus missing; re-run ./scripts/fetch-feedparser.sh")
	}

	// Documents whose root is not feed/rss/RDF are outside the README's claim
	// (RSS 0.90-2.0 + Atom 0.3/1.0): counted and printed, never asserted.
	var wf []string
	for _, p := range wfAll {
		if isFeedRoot(readFile(t, p)) {
			wf = append(wf, p)
		}
	}
	outOfClaim := len(wfAll) - len(wf)

	j := conformParser(t, "atom")

	t.Run("well-formed documents must PARSE", func(t *testing.T) {
		var fails []string
		for _, p := range wf {
			got, err := safeParse(j, readFile(t, p))
			if err != nil {
				fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			if af, ok := got.(tabnasfeed.AtomFeed); !ok || af.Format != "atom" {
				fails = append(fails, fmt.Sprintf("%s: not an atom-shaped result (%T)",
					relPath(t, p), got))
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
		supported, correct, unmappedFiles := 0, 0, 0
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
				correct++
			}
		}
		checked := correct + len(fails)
		fmt.Printf("feedparser values: %d/%d correct (%d of %d files have a machine-checkable Expect; "+
			"%d use paths this harness does not map — known gap)\n  top unmapped paths: %s\n",
			correct, checked, supported, len(wf), unmappedFiles, topCounts(unmapped, 15))

		if checked < feedParserValueCheckedFloor {
			t.Fatalf("only %d value assertions were evaluated, was %d. Value checks "+
				"were LOST — fix the mapping, do not lower the floor.",
				checked, feedParserValueCheckedFloor)
		}
		if correct < feedParserValueCorrectFloor {
			t.Fatalf("%d/%d upstream value assertions hold, was %d. This is a REGRESSION.\n"+
				"Sample of the %d current failures:\n%s",
				correct, checked, feedParserValueCorrectFloor, len(fails), fmtFails(fails))
		}
		if correct > feedParserValueCorrectFloor {
			fmt.Printf("NOTE: value conformance improved to %d/%d; raise "+
				"feedParserValueCorrectFloor (and the TS twin) to %d.\n",
				correct, checked, correct)
		}
	})

	t.Run("ill-formed documents must be REJECTED", func(t *testing.T) {
		var unexpectedlyAccepted, noLongerAccepted []string
		for _, p := range illAll {
			key := strings.TrimPrefix(relPath(t, p), feedParserIllPrefix)
			_, listed := feedParserAcceptedIllformed[key]
			_, err := safeParse(j, readFile(t, p))
			if err == nil && !listed {
				unexpectedlyAccepted = append(unexpectedlyAccepted,
					"ACCEPTED but upstream marks it ill-formed: "+key)
			}
			if err != nil && listed {
				noLongerAccepted = append(noLongerAccepted,
					"now REJECTED (good) — delete its entry from feedParserAcceptedIllformed: "+key)
			}
		}
		fmt.Printf("feedparser illformed: %d/%d rejected (%d enumerated as outside a "+
			"string-input XML parser's reach)\n",
			len(illAll)-len(feedParserAcceptedIllformed), len(illAll),
			len(feedParserAcceptedIllformed))
		if len(unexpectedlyAccepted) > 0 {
			t.Fatalf("must-reject failures:\n%s", fmtFails(unexpectedlyAccepted))
		}
		if len(noLongerAccepted) > 0 {
			t.Fatalf("feedParserAcceptedIllformed is now stale:\n%s", fmtFails(noLongerAccepted))
		}
	})

	// Dialect / version detection. Both oracles come from the corpus, never
	// from us: the document element read out of the source text, and the
	// upstream `Expect: ... version == 'X'` annotation.
	jraw := conformParser(t, "raw")

	t.Run("dialect matches the document element", func(t *testing.T) {
		want := map[string]string{"feed": "atom", "rss": "rss", "RDF": "rdf"}
		var fails []string
		for _, p := range wf {
			src := readFile(t, p)
			root, err := safeParse(jraw, src)
			if err != nil {
				fails = append(fails, relPath(t, p)+": "+strings.Split(err.Error(), "\n")[0])
				continue
			}
			if got := tabnasfeed.Detect(root); got.Dialect != want[rootLocalName(src)] {
				fails = append(fails, fmt.Sprintf("%s: %s (root <%s>)",
					relPath(t, p), got.Dialect, rootLocalName(src)))
			}
		}
		fmt.Printf("feedparser dialect: %d/%d correct\n", len(wf)-len(fails), len(wf))
		if len(fails) > 0 {
			t.Fatalf("dialect failures (%d/%d):\n%s", len(fails), len(wf), fmtFails(fails))
		}
	})

	t.Run("upstream version annotations hold", func(t *testing.T) {
		reVer := regexp.MustCompile(`version == '([a-z0-9]+)'`)
		var fails, staleEntries []string
		checked := 0
		for _, p := range wf {
			src := readFile(t, p)
			m := reVer.FindStringSubmatch(src)
			if m == nil {
				continue
			}
			checked++
			key := strings.TrimPrefix(relPath(t, p), feedParserWfPrefix)
			got := ""
			if root, err := safeParse(jraw, src); err != nil {
				got = "THREW: " + strings.Split(err.Error(), "\n")[0]
			} else {
				got = tabnasfeed.Detect(root).Version
			}
			known, listed := feedParserVersionKnownWrong[key]
			switch {
			case got == m[1]:
				if listed {
					staleEntries = append(staleEntries,
						"now correct — delete its feedParserVersionKnownWrong entry: "+key)
				}
			case known != got:
				extra := ""
				if listed {
					extra = fmt.Sprintf(" (recorded as %s)", known)
				}
				fails = append(fails, fmt.Sprintf("%s: %s, upstream says %s%s",
					key, got, m[1], extra))
			}
		}
		if checked == 0 {
			t.Fatal("no version annotations found — corpus wrong?")
		}
		fmt.Printf("feedparser version: %d/%d correct (%d enumerated disagreements)\n",
			checked-len(feedParserVersionKnownWrong), checked,
			len(feedParserVersionKnownWrong))
		if len(fails) > 0 {
			t.Fatalf("version failures (%d/%d):\n%s", len(fails), checked, fmtFails(fails))
		}
		if len(staleEntries) > 0 {
			t.Fatalf("feedParserVersionKnownWrong is stale:\n%s", fmtFails(staleEntries))
		}
	})
}

// topCounts renders the n most frequent keys of a tally, ties broken by key
// so the line is stable between runs (and between runtimes).
func topCounts(counts map[string]int, n int) string {
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range counts {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(a, b int) bool {
		if all[a].v != all[b].v {
			return all[a].v > all[b].v
		}
		return all[a].k < all[b].k
	})
	if len(all) > n {
		all = all[:n]
	}
	var out []string
	for _, e := range all {
		out = append(out, fmt.Sprintf("%s(%d)", e.k, e.v))
	}
	return strings.Join(out, " ")
}

// jsonShape round-trips a parse result to map[string]any so resolveExpect can
// be the same logic as the TypeScript one: the Go struct json tags ARE the TS
// property names, so going through JSON is what makes the two evaluators
// comparable rather than merely similar.
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
		if i, ok := n(0); ok && contains([]string{"href", "rel", "type", "title"}, s(1)) {
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
		if i, ok := n(0); ok && contains([]string{"term", "scheme", "label"}, s(1)) {
			return mget(aget(mget(obj, "categories"), i), s(1)), ""
		}
	case k == "generator" && len(tail) == 0:
		return mget(mget(obj, "generator"), "value"), ""
	case k == "content" && len(tail) == 2:
		if i, ok := n(0); ok && contains([]string{"value", "type"}, s(1)) {
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
