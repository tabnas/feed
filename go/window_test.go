package tabnasfeed_test

// A parse from a reader, a window of the source at a time (admin ADR-30;
// tabnas/parser's doc/chunked-source.md, "Proving it"), against the same
// text parsed whole. Make declares Lex.Window: the tokens are xml's
// matcher's, which reads the text only through the lexer's window, and this
// plugin's one hook converts the finished element tree, so a reader is read
// a window at a time.
//
// Every row of the shared fixtures and every document of the three feed
// corpora (feedparser-wellformed, feedvalidator and feedparser) is parsed
// whole and from a reader that hands its bytes over in pieces of 1, 3 and
// 64 bytes. The engine's window sizes are fixed in Go (only its own tests
// can make them a few bytes), and the window reaches 64 KiB past the cursor
// before each token is cut, so the edge is proved with a long feed: its
// tokens of every kind straddle the reads, and the long ones, longer than
// the window, run past its end, in pieces of 61, 4096 and 65536 bytes. The
// value, the tokens the lexer gives, the rule passes, and an error's code,
// position, row, column and token must be the same. An error's excerpt is
// not compared: a parse from a reader shows a long line only as far back as
// the window reaches (tabnas/parser's DIVERGENCE.md).
//
// ts/test/window.test.ts and rs/tests/window_test.rs hold the TypeScript
// and Rust ports to the same, under windows of a few units too.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	tabnasfeed "github.com/tabnas/feed/go"
	tabnas "github.com/tabnas/parser/go"
	support "github.com/tabnas/support/go"
)

// pieces is a reader that hands its bytes over at most size at a time.
type pieces struct {
	bytes []byte
	at    int
	size  int
}

func (p *pieces) Read(buf []byte) (int, error) {
	if p.at >= len(p.bytes) {
		return 0, io.EOF
	}
	n := min(p.size, len(buf), len(p.bytes)-p.at)
	copy(buf, p.bytes[p.at:p.at+n])
	p.at += n
	return n, nil
}

// readSizes are the pieces a reader hands a fixture row or a corpus
// document over in, and longSizes a long document's: a read of a few bytes
// costs the Go window's length (tabnas/parser#321), so a long document is
// read in pieces of 61 bytes and more.
var (
	readSizes = []int{1, 3, 64}
	longSizes = []int{61, 4096, 65536}
)

// windowLog gathers the tokens the lexer gives and the rule passes.
type windowLog struct {
	tokens []string
	rules  []string
}

// logged subscribes log to j's tokens and rule passes.
func logged(j *tabnas.Tabnas, log *windowLog) *tabnas.Tabnas {
	j.Sub(func(tkn *tabnas.Token, _ *tabnas.Rule, _ *tabnas.Context) {
		val, _ := json.Marshal(tkn.Val)
		log.tokens = append(log.tokens, fmt.Sprintf("%s %q %s %d %d:%d",
			tkn.Name, tkn.Src, val, tkn.SI, tkn.RI, tkn.CI))
	}, nil)
	j.SubRuleDone(func(r *tabnas.Rule, _ *tabnas.Context, done tabnas.RuleDone) {
		log.rules = append(log.rules, fmt.Sprintf("%s %v %d", r.Name, done.State, r.D))
	})
	return j
}

// outcome is a parse's value as JSON, or its error's code, position, row,
// column and token.
func outcome(value any, err error) string {
	if err != nil {
		var te *tabnas.TabnasError
		if errors.As(err, &te) {
			return fmt.Sprintf("err %s at %d %d:%d %q", te.Code, te.Pos, te.Row, te.Col, te.Src)
		}
		return "fail " + err.Error()
	}
	text, err := json.Marshal(value)
	if err != nil {
		return "unmarshallable " + err.Error()
	}
	return "ok " + string(text)
}

// seenParse is what a parse showed, and the tokens and rule passes the log
// gathered, which is empty again after.
type seenParse struct {
	outcome string
	tokens  []string
	rules   []string
}

func seen(log *windowLog, value any, err error) seenParse {
	got := seenParse{outcome: outcome(value, err), tokens: log.tokens, rules: log.rules}
	log.tokens, log.rules = nil, nil
	return got
}

// differential parses text whole and from a reader in each of sizes, and
// fails t where the two differ. It returns the count of reader parses
// compared.
func differential(t *testing.T, j *tabnas.Tabnas, log *windowLog, at, text string, sizes []int) int {
	t.Helper()
	log.tokens, log.rules = nil, nil
	value, err := j.Parse(text)
	whole := seen(log, value, err)
	for _, size := range sizes {
		value, err := j.ParseReader(&pieces{bytes: []byte(text), size: size})
		read := seen(log, value, err)
		if read.outcome != whole.outcome {
			t.Fatalf("%s in pieces of %d:\nread  %.300s\nwhole %.300s", at, size, read.outcome, whole.outcome)
		}
		if !reflect.DeepEqual(read.tokens, whole.tokens) {
			t.Fatalf("%s in pieces of %d: the tokens differ", at, size)
		}
		if !reflect.DeepEqual(read.rules, whole.rules) {
			t.Fatalf("%s in pieces of %d: the rule passes differ", at, size)
		}
	}
	return len(sizes)
}

// longFeed is a feed of every kind of token, the long ones longer than the
// engine's 64 KiB window, and characters past ASCII among them, so that
// tokens of every kind straddle the window's edges as a reader hands the
// text over. The same document as longFeed in ts/test/window.test.ts and
// long_feed in rs/tests/window_test.rs.
func longFeed() string {
	var b strings.Builder
	b.WriteString("\uFEFF<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	b.WriteString("<!DOCTYPE feed [\n")
	for i := 0; i < 1300; i++ {
		fmt.Fprintf(&b, "  <!ENTITY e%d \"entity \u20ac %d\"> <!-- %d > ] ' -->\n", i, i, i)
	}
	b.WriteString("]>\n")
	b.WriteString("<!-- " + strings.Repeat("a comment past the window, \u00e9 \U0001d11e - ", 2000) + "-->\n")
	b.WriteString("<feed xmlns=\"http://www.w3.org/2005/Atom\" xml:lang=\"en\">\n")
	b.WriteString("  <title type=\"text\">A long feed &amp; its entries</title>\n")
	b.WriteString("  <id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>\n")
	b.WriteString("  <updated>2026-11-09T12:00:00Z</updated>\n")
	b.WriteString("  <link rel=\"self\" href=\"http://example.org/feed?" +
		strings.Repeat("q=a%20long%20query%20past%20the%20window&amp;", 1600) + "\"/>\n")
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "  <entry>\n    <title>Entry %d &#%d; t\u00ebxt</title>\n"+
			"    <id>urn:uuid:%d</id>\n    <updated>2026-11-09T12:00:00Z</updated>\n"+
			"    <author><name>Au\u00f0or %d</name></author>\r\n"+
			"    <summary type=\"html\">&lt;p&gt;summary %d&lt;/p&gt;</summary>\n  </entry>\n",
			i, 60+i%40, i, i, i)
		if i == 500 {
			b.WriteString("  <entry>\n    <title>The long one</title>\n    <id>urn:uuid:long</id>\n" +
				"    <updated>2026-11-09T12:00:00Z</updated>\n" +
				"    <content type=\"html\"><![CDATA[" +
				strings.Repeat("<b>cdata past the window</b> \u00e7 ]] > ", 2200) + "]]></content>\n" +
				"    <summary>" +
				strings.Repeat("character data past the window, \u00e5 &amp; &lt;x&gt; \r\n", 1300) +
				"</summary>\n" +
				"    <?pi " + strings.Repeat("an instruction past the window \u00f1 ? > ", 1800) + "?>\n" +
				"    <category term=\"t\" label=\"" +
				strings.Repeat("a label past the window \u00fc ", 2600) + "\"/>\n" +
				"  </entry>\n")
		}
	}
	b.WriteString("</feed>\n<!-- after -->\n")
	return b.String()
}

func TestMakeReadsAReaderAWindowAtATime(t *testing.T) {
	declares := func(j *tabnas.Tabnas) bool {
		lex := j.Options().Lex
		return lex != nil && lex.Window != nil && *lex.Window
	}
	for _, options := range []map[string]any{nil, {"format": "native"}} {
		j, err := tabnasfeed.Make(options)
		if err != nil {
			t.Fatal(err)
		}
		if !declares(j) {
			t.Fatalf("Make(%v) does not declare Lex.Window", options)
		}
	}
	// The plugin itself declares nothing.
	j := tabnas.Make()
	if err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults); err != nil {
		t.Fatal(err)
	}
	if declares(j) {
		t.Fatal("the plugin declares Lex.Window")
	}
}

func TestEveryFixtureRowParsesTheSameFromAReader(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}
	files, err := support.LoadSpecDir(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	log := &windowLog{}
	rows, compared := 0, 0
	for _, file := range files {
		mode := file.Header[1]
		for _, row := range file.Rows {
			opts := map[string]any{}
			if raw := row.Named("opts"); "" != strings.TrimSpace(raw) {
				if err := json.Unmarshal([]byte(raw), &opts); err != nil {
					t.Fatalf("%s: the opts column is not JSON: %v", row.Where(), err)
				}
			}
			if "detect" == mode {
				opts = map[string]any{"format": "raw"}
			}
			j, err := tabnasfeed.Make(opts)
			if err != nil {
				t.Fatal(err)
			}
			compared += differential(t, logged(j, log), log, row.Where(), row.UnescNamed("input"), readSizes)
			rows++
		}
	}
	if rows < 50 || compared != rows*len(readSizes) {
		t.Fatalf("%d rows, %d reader parses compared", rows, compared)
	}
}

func TestEveryCorpusDocumentParsesTheSameFromAReader(t *testing.T) {
	requireWellformed(t)
	paths := walkXml(t, wellformedDir(t))
	paths = append(paths, walkXml(t, requireCorpus(t, "feedvalidator", "fetch-feedvalidator.sh"))...)
	paths = append(paths, walkXml(t, requireCorpus(t, "feedparser", "fetch-feedparser.sh"))...)
	if len(paths) < 3500 {
		t.Fatalf("%d corpus documents", len(paths))
	}
	log := &windowLog{}
	j, err := tabnasfeed.Make()
	if err != nil {
		t.Fatal(err)
	}
	logged(j, log)
	compared := 0
	for index, path := range paths {
		// A byte that is not UTF-8 is read as U+FFFD, as a reader of the
		// same text gives none.
		text := readFile(t, path)
		if !utf8.ValidString(text) {
			text = strings.ToValidUTF8(text, "\uFFFD")
		}
		compared += differential(t, j, log, relPath(t, path), text, readSizes)
		if index%1000 == 0 || index+1 == len(paths) {
			t.Logf("window corpora: %d of %d documents (%d%%), %d reader parses",
				index+1, len(paths), (index+1)*100/len(paths), compared)
		}
	}
}

func TestALongFeedParsesTheSameFromAReader(t *testing.T) {
	log := &windowLog{}
	j, err := tabnasfeed.Make()
	if err != nil {
		t.Fatal(err)
	}
	text := longFeed()
	if len(text) < 512*1024 {
		t.Fatalf("the long feed is %d bytes", len(text))
	}
	value, err := j.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if feed, ok := value.(tabnasfeed.AtomFeed); !ok || len(feed.Entries) != 1001 {
		t.Fatalf("the long feed parses to %T", value)
	}
	differential(t, logged(j, log), log, "the long feed", text, longSizes)
}

func TestALongDocumentFailsTheSameFromAReader(t *testing.T) {
	// A construct that runs past the engine's window and is then refused,
	// and a long document that is no feed: the error is the whole text's.
	log := &windowLog{}
	j, err := tabnasfeed.Make()
	if err != nil {
		t.Fatal(err)
	}
	logged(j, log)
	pad := "<title>" + strings.Repeat("x\u00e9 ", 30000) + "</title>"
	bad := []string{
		`<feed xmlns="http://www.w3.org/2005/Atom">` + pad + "<!-- " + strings.Repeat("c ", 40000),
		`<feed xmlns="http://www.w3.org/2005/Atom">` + pad + "<subtitle>" +
			strings.Repeat("t ", 40000) + "]]></subtitle></feed>",
		"<kml>" + pad + "</kml>",
		`<rss version="2.0"><channel>` + pad + "</channel></rss>" + strings.Repeat("trailing ", 10000),
	}
	for index, text := range bad {
		if _, err := j.Parse(text); err == nil {
			t.Fatalf("long error document %d parses", index)
		}
		differential(t, j, log, fmt.Sprintf("long error document %d", index), text, longSizes)
	}
}
