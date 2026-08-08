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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
