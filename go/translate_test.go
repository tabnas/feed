package tabnasfeed

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// What the package hands a host is the repository's manifest and the
// alchemy files it names, the render, the embedding and the transducer
// part: npm run embed, from ts/, copies them into go/translate/, and this
// fails until the copies are the files.
func TestTranslationParts(t *testing.T) {
	inCheckout(t)
	parts := Translate()
	if parts == nil {
		t.Fatal("Translate returned nil")
	}
	manifest, err := os.ReadFile("../tabnas.plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	if parts.Manifest != string(manifest) {
		t.Fatal("embedded manifest differs from tabnas.plugin.json")
	}
	if parts.Lift != nil {
		t.Fatal("The feed format has no lift")
	}
	if parts.Render == nil || parts.Render.Entry != "feed-render" {
		t.Fatalf("render entry is %#v", parts.Render)
	}
	render, err := os.ReadFile("../alchemy/render.alc")
	if err != nil {
		t.Fatal(err)
	}
	if parts.Render.Source != string(render) {
		t.Fatal("embedded render differs from alchemy/render.alc")
	}
	if parts.Embed == nil || parts.Embed.Entry != "feed-embed" {
		t.Fatalf("embed entry is %#v", parts.Embed)
	}
	embed, err := os.ReadFile("../alchemy/embed.alc")
	if err != nil {
		t.Fatal(err)
	}
	if parts.Embed.Source != string(embed) {
		t.Fatal("embedded embed differs from alchemy/embed.alc")
	}
	if parts.Transduce == nil || parts.Transduce.Entry != "feed-transduce" {
		t.Fatalf("transduce entry is %#v", parts.Transduce)
	}
	transduce, err := os.ReadFile("../alchemy/transduce.alc")
	if err != nil {
		t.Fatal(err)
	}
	if parts.Transduce.Source != string(transduce) {
		t.Fatal("embedded transduce differs from alchemy/transduce.alc")
	}
}

// The manifest's translate object names the files the package carries,
// and says what the render takes: a tree whose root is object, of the
// schema feed, which a plain tree reaches through the embedding. A host
// that reads parser events reads the tree through the transducer part as
// the passes come, so the manifest gives no reason for reading a document
// whole.
func TestTranslationManifest(t *testing.T) {
	var spec struct {
		LanguageID string `json:"languageId"`
		Translate  struct {
			Reads     string   `json:"reads"`
			Writes    string   `json:"writes"`
			Root      string   `json:"root"`
			Schema    string   `json:"schema"`
			Lift      *string  `json:"lift"`
			Embed     string   `json:"embed"`
			Render    string   `json:"render"`
			Transduce string   `json:"transduce"`
			Whole     *string  `json:"whole"`
			Loss      []string `json:"loss"`
		} `json:"translate"`
	}
	if err := json.Unmarshal([]byte(Translate().Manifest), &spec); err != nil {
		t.Fatal(err)
	}
	tr := spec.Translate
	if spec.LanguageID != "feed" || tr.Reads != "tree" || tr.Writes != "tree" || tr.Root != "object" || tr.Schema != "feed" || tr.Lift != nil {
		t.Fatalf("the manifest says %+v", spec)
	}
	if tr.Embed != "alchemy/embed.alc" || tr.Render != "alchemy/render.alc" || tr.Transduce != "alchemy/transduce.alc" {
		t.Fatalf("the manifest names %q, %q and %q", tr.Embed, tr.Render, tr.Transduce)
	}
	if tr.Whole != nil {
		t.Fatalf("the manifest says the feed is read whole: %q", *tr.Whole)
	}
	if len(tr.Loss) == 0 {
		t.Fatal("the manifest declares no loss")
	}
	for _, line := range tr.Loss {
		if line == "" || line[0] < 'A' || line[0] > 'Z' || line[len(line)-1] != '.' {
			t.Fatalf("%q is not a sentence", line)
		}
	}
}

// Every call hands back parts of its own: a caller that changes what it
// was given, the parts or a part they point to, changes nothing the next
// caller reads, and callers on several goroutines share nothing to race on.
func TestTranslateReturnsACopy(t *testing.T) {
	first := Translate()
	want := *Translate().Render
	first.Manifest = ""
	if first.Render != nil {
		first.Render.Entry = ""
		first.Render.Source = ""
	}
	first.Lift, first.Embed, first.Render, first.Transduce = nil, nil, nil, nil
	second := Translate()
	if second.Manifest == "" {
		t.Fatal("a change to one call's manifest reached the next call")
	}
	if second.Render == nil || *second.Render != want {
		t.Fatalf("a change to one call's render reached the next call: %#v", second.Render)
	}
}

// A host links the render, the embedding and the transducer part with its
// own program and other formats' parts into one namespace, so every
// definition of the three is named for the feed format, the transducer's
// feed-transduce-..., every entry point is there, and no name is defined
// by two of them.
func TestTranslationPartsAreLibrariesNamedForFeed(t *testing.T) {
	parts := Translate()
	render := definitions(parts.Render.Source)
	embed := definitions(parts.Embed.Source)
	transduce := definitions(parts.Transduce.Source)
	for _, want := range []struct {
		names []string
		entry string
	}{{render, "feed-render"}, {embed, "feed-embed"}, {embed, "feed-unembed"}, {transduce, "feed-transduce"}} {
		if !hasName(want.names, want.entry) {
			t.Fatalf("%s is not defined", want.entry)
		}
	}
	seen := map[string]bool{}
	for _, names := range [][]string{render, embed, transduce} {
		for _, name := range names {
			if !strings.HasPrefix(name, "feed-") {
				t.Fatalf("%s is not named for feed", name)
			}
			if seen[name] {
				t.Fatalf("%s is defined by two parts", name)
			}
			seen[name] = true
		}
	}
	for _, name := range transduce {
		if !strings.HasPrefix(name, "feed-transduce") {
			t.Fatalf("%s is not named for the transducer part", name)
		}
	}
}

// definitions is the names a library of alchemy definitions defines, in
// its order.
func definitions(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(line, "def "); ok {
			if fields := strings.Fields(rest); len(fields) > 0 {
				names = append(names, fields[0])
			}
		}
	}
	return names
}

// hasName is whether names holds name.
func hasName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// inCheckout skips a test that holds the embedded copies to the
// repository's own files when it runs where those files are not, as from
// the module cache, whose zip holds the go/ module alone. In a checkout,
// a missing file still fails the test that reads it.
func inCheckout(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("../ts/package.json"); errors.Is(err, fs.ErrNotExist) {
		t.Skip("not in a checkout of the repository: the module cache holds the go/ module alone")
	}
}
