// Copyright (c) 2025 Richard Rodger and other contributors, MIT License

package tabnasfeed_test

// parity_test.go — cross-runtime conformance, driven by the shared
// `test/spec/*.tsv` fixtures at the repo root (see ../test/AGENTS.md).
//
// The fixture loader, the escape codec, the ERROR: contract and the row
// loop all come from github.com/tabnas/support/go, whose TypeScript half
// ts/test/parity.test.ts uses to run the SAME files — so the two
// implementations cannot drift without one of them going red, and neither
// can the two loaders.
//
// What is left here is only what is specific to feed: the two fixture
// modes, and what an ERROR: cell means.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tabnasfeed "github.com/tabnas/feed/go"
	jsonic "github.com/tabnas/jsonic/go"
	support "github.com/tabnas/support/go"
)

// TestSpec runs every fixture in the spec directory. A fixture's SECOND
// COLUMN HEADER says what it asserts: `expected` is the parsed feed,
// `detect` is the format name the raw parse is recognised as. That is per
// file, which is why there is a runner per file rather than one over the
// directory.
func TestSpec(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}

	specs, err := support.LoadSpecDir(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, spec := range specs {
		if 2 > len(spec.Header) {
			t.Fatalf("%s: expected at least two columns", spec.Name)
		}
		mode := spec.Header[1]
		if "expected" != mode && "detect" != mode {
			t.Fatalf("%s: unknown second column %q", spec.Name, mode)
		}

		support.Runner{
			ParseRow: func(input string, row *support.Row) (any, error) {
				opts := map[string]any{}
				if raw := row.Named("opts"); "" != strings.TrimSpace(raw) {
					if err := json.Unmarshal([]byte(raw), &opts); err != nil {
						return nil, err
					}
				}
				// Detection is asserted over the RAW parse, so those
				// fixtures pin the format rather than passing their own
				// options.
				if "detect" == mode {
					opts = map[string]any{"format": "raw"}
				}

				j := jsonic.Make()
				if err := j.UseDefaults(
					tabnasfeed.Feed, tabnasfeed.Defaults, opts); err != nil {
					return nil, err
				}

				parsed, err := j.Parse(input)
				if err != nil {
					return nil, err
				}
				if "detect" == mode {
					return tabnasfeed.Detect(parsed), nil
				}
				return parsed, nil
			},

			// feed's ERROR:<want> cells hold a fragment of the MESSAGE —
			// `unrecognized root element "kml"`, `character data is not
			// allowed outside the root element` — rather than an error
			// code. These rejections come from the feed layer's own
			// validation, which reports what is wrong in prose rather than
			// through a code the engine assigns. A bare ERROR still accepts
			// any failure.
			MatchError: func(err error, want string, _ *support.Row) bool {
				return strings.Contains(err.Error(), want)
			},

			// Flatten through JSON so class identity and field order do not
			// affect the structural comparison.
			Normalize: jsonFlatten,

			ExpectedName: mode,
			CaseName: func(row *support.Row, input string) string {
				return fmt.Sprintf("row %d: %q", row.Line, input)
			},
		}.Spec(t, spec)
	}
}

// jsonFlatten renders a value as JSON and reads it back as plain
// map/slice/float64/string/bool/nil. A value that will not marshal is
// returned as it is: the comparison then fails and prints it, which says
// more than a panic here would.
func jsonFlatten(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}
