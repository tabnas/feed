// Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License

package feed_test

import (
	"testing"
	"time"

	jsonic "github.com/tabnas/jsonic/go"
	feed "github.com/tabnas/feed/go"
)

// makeFeedParser builds a fresh Jsonic instance with the Feed plugin in the
// default (atom) format. Building this is expensive: Feed pulls in the xml
// plugin and rebuilds the whole grammar, which dominates a parse.
func makeFeedParser() *jsonic.Jsonic {
	j := jsonic.Make()
	_ = j.UseDefaults(feed.Feed, feed.Defaults)
	return j
}

// TestParseReusesInstance guards against a performance regression. The feed
// package has NO package-level convenience Parse(): callers instantiate the
// plugin themselves (jsonic.Make() + UseDefaults(feed.Feed, ...)) and are
// expected to REUSE that instance across parses. Building the instance
// (xml plugin + grammar) dominates a parse, so rebuilding it on every parse
// is many times slower than reusing one instance.
//
// This test pins that expectation: it times N parses that rebuild a fresh
// instance each iteration against N parses that reuse a single instance, on
// the SAME machine in the SAME run. The comparison is machine-INDEPENDENT
// (both sides scale together on a slow CI box) and there is deliberately NO
// wall-clock budget. If someone later adds a convenience Parse() that
// rebuilds the grammar per call — the bug fixed across these plugins — a test
// of that shape would catch it the same way.
func TestParseReusesInstance(t *testing.T) {
	const src = `<rss version="2.0"><channel><title>x</title>` +
		`<item><title>i</title></item></channel></rss>`
	const n = 300

	// Warm both paths so the comparison is steady-state.
	for i := 0; i < 20; i++ {
		if _, err := makeFeedParser().Parse(src); err != nil {
			t.Fatalf("warm rebuild parse error: %v", err)
		}
	}
	reused := makeFeedParser()
	for i := 0; i < 20; i++ {
		if _, err := reused.Parse(src); err != nil {
			t.Fatalf("warm reuse parse error: %v", err)
		}
	}

	// Rebuild-per-parse: the anti-pattern (what a non-cached convenience
	// Parse() would do internally).
	t0 := time.Now()
	for i := 0; i < n; i++ {
		if _, err := makeFeedParser().Parse(src); err != nil {
			t.Fatalf("rebuild parse error: %v", err)
		}
	}
	rebuild := time.Since(t0)

	// Reuse one instance: the documented, efficient usage.
	t1 := time.Now()
	for i := 0; i < n; i++ {
		if _, err := reused.Parse(src); err != nil {
			t.Fatalf("reuse parse error: %v", err)
		}
	}
	reuse := time.Since(t1)

	// Reusing an instance must be far cheaper than rebuilding it per parse.
	// Building the grammar dominates, so rebuild is many times slower here;
	// requiring at least a 4x speedup catches a regression where instance
	// reuse stops paying off (e.g. the grammar is rebuilt on every Parse)
	// without depending on absolute wall-clock speed.
	if reuse*4 > rebuild {
		t.Errorf("reusing a feed parser is not meaningfully faster than "+
			"rebuilding one per parse: %d reuse parses took %v vs %v rebuilding "+
			"each time (speedup %.1fx, want >=4x). Building the instance "+
			"(xml plugin + grammar) should dominate, so reuse should win big; "+
			"reuse a single instance across parses (see makeFeedParser).",
			n, reuse, rebuild, float64(rebuild)/float64(reuse))
	}
	t.Logf("rebuild-per-parse=%v  reuse=%v  speedup=%.2fx", rebuild, reuse, float64(rebuild)/float64(reuse))
}
