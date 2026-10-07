// Copyright (c) 2026 Richard Rodger and other contributors, MIT License

package tabnasfeed

// The attributes of an XML element reach feed in one of two shapes, and the
// helpers read both. github.com/tabnas/xml/go gives a *tabnas.OrderedMap in
// source order from the release after 0.7.14; up to 0.7.14 it gave a plain
// map[string]any. These tests build the trees by hand, so they hold
// whichever release go.mod names.

import (
	"testing"

	jsonic "github.com/tabnas/jsonic/go"
	tabnas "github.com/tabnas/parser/go"
)

func TestSerializeElementWritesOrderedAttributesInOrder(t *testing.T) {
	attrs := tabnas.NewOrderedMap()
	attrs.Set("title", "T")
	attrs.Set("href", `x&"<>`)
	attrs.Set("class", "c")
	el := map[string]any{
		"name": "a", "localName": "a", "attributes": attrs,
		"children": []any{"link"},
	}
	want := `<a title="T" href="x&amp;&quot;&lt;&gt;" class="c">link</a>`
	if got := serializeElement(el); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A plain map has no order, and ranging over one starts at a random place
// each time, so its attributes are written sorted by name: the same text on
// every call.
func TestSerializeElementSortsPlainMapAttributes(t *testing.T) {
	for _, el := range []map[string]any{
		{"name": "img", "attributes": map[string]any{
			"src": "i.png", "alt": "A", "width": "1", "height": "2", "class": "c",
		}, "children": []any{}},
		{"name": "img", "attributes": map[string]string{
			"src": "i.png", "alt": "A", "width": "1", "height": "2", "class": "c",
		}, "children": []any{}},
	} {
		want := `<img alt="A" class="c" height="2" src="i.png" width="1"/>`
		for i := 0; i < 20; i++ {
			if got := serializeElement(el); got != want {
				t.Fatalf("%T, call %d:\ngot  %s\nwant %s", el["attributes"], i+1, got, want)
			}
		}
	}
}

func TestAttributeReadsEveryShape(t *testing.T) {
	ordered := tabnas.NewOrderedMap()
	ordered.Set("version", "2.0")
	var nilOrdered *tabnas.OrderedMap
	for _, c := range []struct {
		attrs any
		want  string
		ok    bool
	}{
		{ordered, "2.0", true},
		{map[string]any{"version": "2.0"}, "2.0", true},
		{map[string]string{"version": "2.0"}, "2.0", true},
		{tabnas.NewOrderedMap(), "", false},
		{nilOrdered, "", false},
		{nil, "", false},
	} {
		got, ok := attribute(map[string]any{"attributes": c.attrs}, "version")
		if got != c.want || ok != c.ok {
			t.Errorf("%T: got (%q, %v), want (%q, %v)", c.attrs, got, ok, c.want, c.ok)
		}
	}
}

// XHTML content is written back from the element tree. Whichever shape the
// attributes come in, the same document gives the same text on every parse.
func TestXhtmlContentIsTheSameOnEveryParse(t *testing.T) {
	const src = `<feed xmlns="http://www.w3.org/2005/Atom"><entry>` +
		`<content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml">` +
		`<a title="T" href="http://example.org/" class="c" id="x" rel="r">link</a>` +
		`<img src="i.png" alt="A" width="1" height="2"/></div></content>` +
		`</entry></feed>`
	j := jsonic.Make()
	if err := j.UseDefaults(Feed, Defaults); err != nil {
		t.Fatal(err)
	}
	first := ""
	for i := 0; i < 30; i++ {
		out, err := j.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		feed, ok := out.(AtomFeed)
		if !ok || len(feed.Entries) != 1 || feed.Entries[0].Content == nil {
			t.Fatalf("unexpected result %#v", out)
		}
		value := feed.Entries[0].Content.Value
		if i == 0 {
			first = value
		} else if value != first {
			t.Fatalf("parse %d wrote\n  %s\nwhere parse 1 wrote\n  %s", i+1, value, first)
		}
	}
}
