// Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License

package tabnasfeed_test

// parity_test.go — cross-runtime conformance, driven by the shared
// `test/spec/*.tsv` fixtures at the repo root (see ../test/AGENTS.md), the
// same convention @tabnas/parser and @tabnas/abnf use.
//
// ts/test/parity.test.ts discovers and runs the SAME files, so the two
// implementations cannot drift without one of them going red.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tabnasfeed "github.com/tabnas/feed/go"
	jsonic "github.com/tabnas/jsonic/go"
)

type specRow struct {
	file     string
	lineNo   int
	input    string
	expected string
	opts     string
}

func specDir() string { return filepath.Join("..", "test", "spec") }

// specUnescape decodes the escape set used in non-JSON columns. Kept
// byte-identical to the TS loader so both runtimes feed the parser the exact
// same source text.
func specUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// loadSpec reads one fixture file. The header row names the columns, and its
// second name selects what the runner compares: `expected` is the parse
// result, `detect` is the dialect report for the raw element tree.
func loadSpec(t *testing.T, path string) (string, []specRow) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	mode := "expected"
	var rows []specRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if lineNo == 1 {
			if cols := strings.Split(line, "\t"); 2 <= len(cols) && cols[1] == "detect" {
				mode = "detect"
			}
			continue
		}
		// A comment line starts with '#' and has no tab; a data row always
		// has at least one (input + expected), so '#'-leading sources still
		// work.
		if line == "" || (strings.HasPrefix(line, "#") && !strings.Contains(line, "\t")) {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			t.Fatalf("%s:%d: expected at least 2 tab-separated columns", path, lineNo)
		}
		row := specRow{
			file:     filepath.Base(path),
			lineNo:   lineNo,
			input:    specUnescape(cols[0]),
			expected: cols[1],
		}
		if 3 <= len(cols) {
			row.opts = cols[2]
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s: no cases", path)
	}
	return mode, rows
}

// specLabel is a truncated single-line rendering of the input, so a failure
// names its case readably.
func specLabel(s string) string {
	one := strings.Join(strings.Fields(s), " ")
	if 60 < len(one) {
		return one[:57] + "..."
	}
	return one
}

// jsonRound normalises through JSON so struct types, *OrderedMap and the
// fixture's decoded shape compare structurally.
func jsonRound(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return out
}

func runSpecFile(t *testing.T, path string) {
	mode, rows := loadSpec(t, path)
	for _, row := range rows {
		t.Run(specLabel(row.input), func(t *testing.T) {
			opts := map[string]any{}
			if strings.TrimSpace(row.opts) != "" {
				if err := json.Unmarshal([]byte(row.opts), &opts); err != nil {
					t.Fatalf("%s:%d: bad opts JSON %q: %v", row.file, row.lineNo, row.opts, err)
				}
			}
			if mode == "detect" {
				opts = map[string]any{"format": "raw"}
			}

			j := jsonic.Make()
			if err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults, opts); err != nil {
				t.Fatalf("plugin init: %v", err)
			}
			parsed, err := j.Parse(row.input)

			if strings.HasPrefix(row.expected, "ERROR") {
				want := strings.TrimPrefix(strings.TrimPrefix(row.expected, "ERROR"), ":")
				if err == nil {
					t.Fatalf("%s:%d: expected error, got %v", row.file, row.lineNo, parsed)
				}
				if want != "" && !strings.Contains(err.Error(), want) {
					t.Fatalf("%s:%d: expected error %q, got %q", row.file, row.lineNo, want, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("%s:%d: unexpected parse error: %v", row.file, row.lineNo, err)
			}

			var value any = parsed
			if mode == "detect" {
				value = tabnasfeed.Detect(parsed)
			}

			var want any
			if err := json.Unmarshal([]byte(row.expected), &want); err != nil {
				t.Fatalf("%s:%d: bad expected JSON %q: %v", row.file, row.lineNo, row.expected, err)
			}
			if gotVal := jsonRound(t, value); !reflect.DeepEqual(gotVal, want) {
				gotJSON, _ := json.Marshal(gotVal)
				wantJSON, _ := json.Marshal(want)
				t.Errorf("%s:%d:\n  got  %s\n  want %s", row.file, row.lineNo, gotJSON, wantJSON)
			}
		})
	}
}

// TestSpec auto-discovers every fixture: adding a .tsv runs it in both
// runtimes without touching either runner.
func TestSpec(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(specDir(), "*.tsv"))
	if err != nil {
		t.Fatalf("glob spec dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no spec files under %s", specDir())
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) { runSpecFile(t, path) })
	}
}
