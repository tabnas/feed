// Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License

// Package feed is a Jsonic plugin that parses RSS (0.90, 0.91, 0.92, 1.0,
// 2.0) and Atom (0.3, 1.0) syndication feeds. Built on top of
// github.com/tabnas/xml/go, every dialect is normalised by default to an
// Atom-shaped result; pass options{"format": "native"} to keep the source
// dialect's structure or options{"format": "raw"} to get back the raw XML
// element tree from the xml plugin.
package feed

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	jsonic "github.com/tabnas/jsonic/go"
	xml "github.com/tabnas/xml/go"
)

const Version = "0.1.0"

// Defaults are merged with caller-supplied options when the plugin is
// registered via jsonic.UseDefaults. The single supported option is
// "format" with values "atom" (default), "native", or "raw".
var Defaults = map[string]any{
	"format": "atom",
}

// --- Public types ---------------------------------------------------------

// AtomText carries an Atom <title|subtitle|summary|content|rights> body.
type AtomText struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// AtomPerson is an author or contributor.
type AtomPerson struct {
	Name  string `json:"name"`
	URI   string `json:"uri,omitempty"`
	Email string `json:"email,omitempty"`
}

// AtomLink is an Atom <link>.
type AtomLink struct {
	Href     string `json:"href"`
	Rel      string `json:"rel,omitempty"`
	Type     string `json:"type,omitempty"`
	Hreflang string `json:"hreflang,omitempty"`
	Title    string `json:"title,omitempty"`
	Length   int    `json:"length,omitempty"`
}

// AtomCategory is an Atom <category>.
type AtomCategory struct {
	Term   string `json:"term"`
	Scheme string `json:"scheme,omitempty"`
	Label  string `json:"label,omitempty"`
}

// AtomGenerator is an Atom <generator>.
type AtomGenerator struct {
	URI     string `json:"uri,omitempty"`
	Version string `json:"version,omitempty"`
	Value   string `json:"value"`
}

// AtomContent is an Atom <content>.
type AtomContent struct {
	Type  string `json:"type"`
	Src   string `json:"src,omitempty"`
	Value string `json:"value,omitempty"`
}

// AtomEntrySource is the slim Atom-feed shape used in AtomEntry.Source. It
// mirrors the TS Partial<AtomFeed> with only the fields produced when an
// RSS <source> element is mapped to Atom.
type AtomEntrySource struct {
	Format  string     `json:"format"`
	Version string     `json:"version"`
	ID      string     `json:"id,omitempty"`
	Title   *AtomText  `json:"title,omitempty"`
	Updated string     `json:"updated,omitempty"`
	Links   []AtomLink `json:"links,omitempty"`
}

// AtomEntry is an Atom <entry>.
type AtomEntry struct {
	ID           string           `json:"id,omitempty"`
	Title        *AtomText        `json:"title,omitempty"`
	Updated      string           `json:"updated,omitempty"`
	Published    string           `json:"published,omitempty"`
	Authors      []AtomPerson     `json:"authors,omitempty"`
	Contributors []AtomPerson     `json:"contributors,omitempty"`
	Categories   []AtomCategory   `json:"categories,omitempty"`
	Content      *AtomContent     `json:"content,omitempty"`
	Links        []AtomLink       `json:"links,omitempty"`
	Rights       *AtomText        `json:"rights,omitempty"`
	Summary      *AtomText        `json:"summary,omitempty"`
	Source       *AtomEntrySource `json:"source,omitempty"`
}

// AtomFeed is the normalised Atom-shaped output (the default).
type AtomFeed struct {
	Format       string         `json:"format"`
	Version      string         `json:"version"`
	ID           string         `json:"id,omitempty"`
	Title        *AtomText      `json:"title,omitempty"`
	Updated      string         `json:"updated,omitempty"`
	Authors      []AtomPerson   `json:"authors,omitempty"`
	Contributors []AtomPerson   `json:"contributors,omitempty"`
	Categories   []AtomCategory `json:"categories,omitempty"`
	Generator    *AtomGenerator `json:"generator,omitempty"`
	Icon         string         `json:"icon,omitempty"`
	Logo         string         `json:"logo,omitempty"`
	Rights       *AtomText      `json:"rights,omitempty"`
	Subtitle     *AtomText      `json:"subtitle,omitempty"`
	Links        []AtomLink     `json:"links,omitempty"`
	Entries      []AtomEntry    `json:"entries"`
}

// Rss2Category is an RSS 2.x <category>.
type Rss2Category struct {
	Domain string `json:"domain,omitempty"`
	Value  string `json:"value"`
}

// Rss2Enclosure is an RSS 2.x <enclosure>.
type Rss2Enclosure struct {
	URL    string `json:"url"`
	Length int    `json:"length,omitempty"`
	Type   string `json:"type,omitempty"`
}

// Rss2Guid is an RSS 2.x <guid>.
type Rss2Guid struct {
	IsPermaLink *bool  `json:"isPermaLink,omitempty"`
	Value       string `json:"value"`
}

// Rss2Source is an RSS 2.x item <source>.
type Rss2Source struct {
	URL   string `json:"url,omitempty"`
	Value string `json:"value"`
}

// Rss2Image is an RSS 2.x channel <image>.
type Rss2Image struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Description string `json:"description,omitempty"`
}

// Rss2Cloud is an RSS 2.x channel <cloud>.
type Rss2Cloud struct {
	Domain            string `json:"domain"`
	Port              int    `json:"port"`
	Path              string `json:"path"`
	RegisterProcedure string `json:"registerProcedure"`
	Protocol          string `json:"protocol"`
}

// Rss2TextInput is an RSS 2.x channel <textInput>.
type Rss2TextInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Name        string `json:"name"`
	Link        string `json:"link"`
}

// Rss2Item is an RSS 2.x channel item.
type Rss2Item struct {
	Title       string         `json:"title,omitempty"`
	Link        string         `json:"link,omitempty"`
	Description string         `json:"description,omitempty"`
	Author      string         `json:"author,omitempty"`
	Categories  []Rss2Category `json:"categories,omitempty"`
	Comments    string         `json:"comments,omitempty"`
	Enclosure   *Rss2Enclosure `json:"enclosure,omitempty"`
	Guid        *Rss2Guid      `json:"guid,omitempty"`
	PubDate     string         `json:"pubDate,omitempty"`
	Source      *Rss2Source    `json:"source,omitempty"`
}

// Rss2Feed is the native shape for RSS 0.91 / 0.92 / 2.0.
type Rss2Feed struct {
	Format         string         `json:"format"`
	Version        string         `json:"version"`
	Title          string         `json:"title"`
	Link           string         `json:"link"`
	Description    string         `json:"description"`
	Language       string         `json:"language,omitempty"`
	Copyright      string         `json:"copyright,omitempty"`
	ManagingEditor string         `json:"managingEditor,omitempty"`
	WebMaster      string         `json:"webMaster,omitempty"`
	PubDate        string         `json:"pubDate,omitempty"`
	LastBuildDate  string         `json:"lastBuildDate,omitempty"`
	Categories     []Rss2Category `json:"categories,omitempty"`
	Generator      string         `json:"generator,omitempty"`
	Docs           string         `json:"docs,omitempty"`
	Cloud          *Rss2Cloud     `json:"cloud,omitempty"`
	TTL            int            `json:"ttl,omitempty"`
	Image          *Rss2Image     `json:"image,omitempty"`
	TextInput      *Rss2TextInput `json:"textInput,omitempty"`
	SkipHours      []int          `json:"skipHours,omitempty"`
	SkipDays       []string       `json:"skipDays,omitempty"`
	Items          []Rss2Item     `json:"items"`
}

// Rss1Item is an RSS 1.0 / 0.90 item.
type Rss1Item struct {
	About       string `json:"about,omitempty"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description,omitempty"`
}

// Rss1Image is an RSS 1.0 / 0.90 image.
type Rss1Image struct {
	About string `json:"about,omitempty"`
	Title string `json:"title"`
	Link  string `json:"link"`
	URL   string `json:"url"`
}

// Rss1TextInput is an RSS 1.0 / 0.90 textinput.
type Rss1TextInput struct {
	About       string `json:"about,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Name        string `json:"name"`
	Link        string `json:"link"`
}

// Rss1Feed is the native shape for RSS 0.90 / 1.0 (RDF).
type Rss1Feed struct {
	Format      string         `json:"format"`
	Version     string         `json:"version"`
	About       string         `json:"about,omitempty"`
	Title       string         `json:"title"`
	Link        string         `json:"link"`
	Description string         `json:"description,omitempty"`
	Image       *Rss1Image     `json:"image,omitempty"`
	TextInput   *Rss1TextInput `json:"textInput,omitempty"`
	Items       []Rss1Item     `json:"items"`
}

// Detection is the result of Detect.
type Detection struct {
	Dialect string `json:"dialect"`
	Version string `json:"version"`
}

// --- Namespaces -----------------------------------------------------------

const (
	nsAtom10 = "http://www.w3.org/2005/Atom"
	nsAtom03 = "http://purl.org/atom/ns#"
	nsRSS10  = "http://purl.org/rss/1.0/"
	nsRSS090 = "http://my.netscape.com/rdf/simple/0.9/"
)

// --- XmlElement helpers ---------------------------------------------------
//
// The xml plugin returns elements as map[string]any with keys:
//   name, localName, prefix, namespace, attributes (map[string]string),
//   children ([]any with strings or nested element maps).

func asElement(v any) (map[string]any, bool) {
	el, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	if _, hasChildren := el["children"]; !hasChildren {
		return nil, false
	}
	return el, true
}

func localName(el map[string]any) string {
	if s, ok := el["localName"].(string); ok {
		return s
	}
	if s, ok := el["name"].(string); ok {
		return s
	}
	return ""
}

func namespace(el map[string]any) string {
	if s, ok := el["namespace"].(string); ok {
		return s
	}
	return ""
}

func attribute(el map[string]any, name string) (string, bool) {
	a, ok := el["attributes"].(map[string]any)
	if !ok {
		// xml plugin usually emits attributes as map[string]string
		if as, ok2 := el["attributes"].(map[string]string); ok2 {
			v, ok3 := as[name]
			return v, ok3
		}
		return "", false
	}
	v, ok := a[name].(string)
	return v, ok
}

func children(el map[string]any) []any {
	c, _ := el["children"].([]any)
	return c
}

func findChild(el map[string]any, name string) map[string]any {
	if el == nil {
		return nil
	}
	for _, c := range children(el) {
		ce, ok := asElement(c)
		if !ok {
			continue
		}
		if localName(ce) == name {
			return ce
		}
	}
	return nil
}

func findChildren(el map[string]any, name string) []map[string]any {
	out := []map[string]any{}
	if el == nil {
		return out
	}
	for _, c := range children(el) {
		ce, ok := asElement(c)
		if !ok {
			continue
		}
		if localName(ce) == name {
			out = append(out, ce)
		}
	}
	return out
}

// textOf concatenates direct text-node children, trimmed.
func textOf(el map[string]any) string {
	if el == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range children(el) {
		if s, ok := c.(string); ok {
			b.WriteString(s)
		}
	}
	return strings.TrimSpace(b.String())
}

// innerXml serialises an element's children (text + nested elements)
// back to a string. Used for xhtml content extraction.
func innerXml(el map[string]any) string {
	if el == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range children(el) {
		if s, ok := c.(string); ok {
			b.WriteString(s)
			continue
		}
		if ce, ok := asElement(c); ok {
			b.WriteString(serializeElement(ce))
		}
	}
	return b.String()
}

func serializeElement(el map[string]any) string {
	name, _ := el["name"].(string)
	if name == "" {
		name = localName(el)
	}
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(name)
	switch attrs := el["attributes"].(type) {
	case map[string]any:
		for k, v := range attrs {
			s, _ := v.(string)
			b.WriteString(` `)
			b.WriteString(k)
			b.WriteString(`="`)
			b.WriteString(escapeAttr(s))
			b.WriteString(`"`)
		}
	case map[string]string:
		for k, v := range attrs {
			b.WriteString(` `)
			b.WriteString(k)
			b.WriteString(`="`)
			b.WriteString(escapeAttr(v))
			b.WriteString(`"`)
		}
	}
	inner := innerXml(el)
	if inner == "" {
		b.WriteString("/>")
		return b.String()
	}
	b.WriteByte('>')
	b.WriteString(inner)
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
	return b.String()
}

var attrReplacer = strings.NewReplacer(
	`&`, `&amp;`,
	`"`, `&quot;`,
	`<`, `&lt;`,
	`>`, `&gt;`,
)

func escapeAttr(s string) string { return attrReplacer.Replace(s) }

// --- Detection ------------------------------------------------------------

// Detect inspects a parsed XML root element and reports the feed dialect /
// version. It accepts the map[string]any element shape produced by the
// xml plugin.
func Detect(root any) Detection {
	el, ok := asElement(root)
	if !ok {
		return Detection{Dialect: "unknown", Version: "unknown"}
	}
	switch localName(el) {
	case "feed":
		if namespace(el) == nsAtom03 {
			return Detection{Dialect: "atom", Version: "atom03"}
		}
		return Detection{Dialect: "atom", Version: "atom10"}

	case "rss":
		v, _ := attribute(el, "version")
		switch v {
		case "2.0", "2.0.1", "2.0.2":
			return Detection{Dialect: "rss", Version: "rss20"}
		case "0.92", "0.93", "0.94":
			return Detection{Dialect: "rss", Version: "rss092"}
		case "0.91":
			return Detection{Dialect: "rss", Version: "rss091u"}
		}
		return Detection{Dialect: "rss", Version: "rss20"}

	case "RDF":
		ch := findChild(el, "channel")
		if ch != nil && namespace(ch) == nsRSS090 {
			return Detection{Dialect: "rdf", Version: "rss090"}
		}
		return Detection{Dialect: "rdf", Version: "rss10"}
	}

	return Detection{Dialect: "unknown", Version: "unknown"}
}

// --- Atom (native) parser -------------------------------------------------

func parsePerson(el map[string]any) AtomPerson {
	p := AtomPerson{Name: textOf(findChild(el, "name"))}
	if uri := textOf(findChild(el, "uri")); uri != "" {
		p.URI = uri
	} else if url := textOf(findChild(el, "url")); url != "" {
		p.URI = url
	}
	if e := textOf(findChild(el, "email")); e != "" {
		p.Email = e
	}
	return p
}

func parseLink(el map[string]any) AtomLink {
	l := AtomLink{}
	if h, ok := attribute(el, "href"); ok {
		l.Href = h
	}
	if v, ok := attribute(el, "rel"); ok {
		l.Rel = v
	}
	if v, ok := attribute(el, "type"); ok {
		l.Type = v
	}
	if v, ok := attribute(el, "hreflang"); ok {
		l.Hreflang = v
	}
	if v, ok := attribute(el, "title"); ok {
		l.Title = v
	}
	if v, ok := attribute(el, "length"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			l.Length = n
		}
	}
	return l
}

func parseCategory(el map[string]any) AtomCategory {
	c := AtomCategory{}
	if t, ok := attribute(el, "term"); ok {
		c.Term = t
	} else {
		c.Term = textOf(el)
	}
	if v, ok := attribute(el, "scheme"); ok {
		c.Scheme = v
	}
	if v, ok := attribute(el, "label"); ok {
		c.Label = v
	}
	return c
}

func parseText(el map[string]any) *AtomText {
	if el == nil {
		return nil
	}
	t := "text"
	if v, ok := attribute(el, "type"); ok {
		t = v
	}
	if t == "xhtml" {
		div := findChild(el, "div")
		if div == nil {
			div = el
		}
		return &AtomText{Type: t, Value: innerXml(div)}
	}
	return &AtomText{Type: t, Value: textOf(el)}
}

func parseContent(el map[string]any) *AtomContent {
	t := "text"
	if v, ok := attribute(el, "type"); ok {
		t = v
	}
	out := &AtomContent{Type: t}
	if v, ok := attribute(el, "src"); ok {
		out.Src = v
	}
	if t == "xhtml" {
		div := findChild(el, "div")
		if div == nil {
			div = el
		}
		out.Value = innerXml(div)
	} else {
		out.Value = textOf(el)
	}
	return out
}

func parseGenerator(el map[string]any) *AtomGenerator {
	g := &AtomGenerator{Value: textOf(el)}
	if v, ok := attribute(el, "uri"); ok {
		g.URI = v
	}
	if v, ok := attribute(el, "version"); ok {
		g.Version = v
	}
	return g
}

func parseAtomEntry(el map[string]any, atom03 bool) AtomEntry {
	entry := AtomEntry{}
	if id := findChild(el, "id"); id != nil {
		entry.ID = textOf(id)
	}
	if t := findChild(el, "title"); t != nil {
		entry.Title = parseText(t)
	}
	updatedTag := "updated"
	publishedTag := "published"
	if atom03 {
		updatedTag = "modified"
		publishedTag = "issued"
	}
	if u := findChild(el, updatedTag); u != nil {
		entry.Updated = textOf(u)
	}
	if p := findChild(el, publishedTag); p != nil {
		entry.Published = textOf(p)
	}
	if as := findChildren(el, "author"); len(as) > 0 {
		entry.Authors = make([]AtomPerson, 0, len(as))
		for _, a := range as {
			entry.Authors = append(entry.Authors, parsePerson(a))
		}
	}
	if cs := findChildren(el, "contributor"); len(cs) > 0 {
		entry.Contributors = make([]AtomPerson, 0, len(cs))
		for _, c := range cs {
			entry.Contributors = append(entry.Contributors, parsePerson(c))
		}
	}
	if cs := findChildren(el, "category"); len(cs) > 0 {
		entry.Categories = make([]AtomCategory, 0, len(cs))
		for _, c := range cs {
			entry.Categories = append(entry.Categories, parseCategory(c))
		}
	}
	if ls := findChildren(el, "link"); len(ls) > 0 {
		entry.Links = make([]AtomLink, 0, len(ls))
		for _, l := range ls {
			entry.Links = append(entry.Links, parseLink(l))
		}
	}
	if c := findChild(el, "content"); c != nil {
		entry.Content = parseContent(c)
	}
	rightsTag := "rights"
	if atom03 {
		rightsTag = "copyright"
	}
	if r := findChild(el, rightsTag); r != nil {
		entry.Rights = parseText(r)
	}
	summaryTag := "summary"
	if atom03 {
		summaryTag = "tagline"
	}
	if s := findChild(el, summaryTag); s != nil {
		entry.Summary = parseText(s)
	}
	return entry
}

func parseAtom(root map[string]any, atom03 bool) AtomFeed {
	feed := AtomFeed{Format: "atom", Version: "1.0", Entries: []AtomEntry{}}
	if atom03 {
		feed.Version = "0.3"
	}
	if id := findChild(root, "id"); id != nil {
		feed.ID = textOf(id)
	}
	if t := findChild(root, "title"); t != nil {
		feed.Title = parseText(t)
	}
	updatedTag := "updated"
	if atom03 {
		updatedTag = "modified"
	}
	if u := findChild(root, updatedTag); u != nil {
		feed.Updated = textOf(u)
	}
	if as := findChildren(root, "author"); len(as) > 0 {
		feed.Authors = make([]AtomPerson, 0, len(as))
		for _, a := range as {
			feed.Authors = append(feed.Authors, parsePerson(a))
		}
	}
	if cs := findChildren(root, "contributor"); len(cs) > 0 {
		feed.Contributors = make([]AtomPerson, 0, len(cs))
		for _, c := range cs {
			feed.Contributors = append(feed.Contributors, parsePerson(c))
		}
	}
	if cs := findChildren(root, "category"); len(cs) > 0 {
		feed.Categories = make([]AtomCategory, 0, len(cs))
		for _, c := range cs {
			feed.Categories = append(feed.Categories, parseCategory(c))
		}
	}
	if ls := findChildren(root, "link"); len(ls) > 0 {
		feed.Links = make([]AtomLink, 0, len(ls))
		for _, l := range ls {
			feed.Links = append(feed.Links, parseLink(l))
		}
	}
	if g := findChild(root, "generator"); g != nil {
		feed.Generator = parseGenerator(g)
	}
	if i := findChild(root, "icon"); i != nil {
		feed.Icon = textOf(i)
	}
	if l := findChild(root, "logo"); l != nil {
		feed.Logo = textOf(l)
	}
	rightsTag := "rights"
	if atom03 {
		rightsTag = "copyright"
	}
	if r := findChild(root, rightsTag); r != nil {
		feed.Rights = parseText(r)
	}
	subtitleTag := "subtitle"
	if atom03 {
		subtitleTag = "tagline"
	}
	if s := findChild(root, subtitleTag); s != nil {
		feed.Subtitle = parseText(s)
	}
	for _, e := range findChildren(root, "entry") {
		feed.Entries = append(feed.Entries, parseAtomEntry(e, atom03))
	}
	return feed
}

// --- RSS 2.x / 0.91 / 0.92 (native) parser --------------------------------

func parseRss2Image(el map[string]any) *Rss2Image {
	img := &Rss2Image{
		URL:   textOf(findChild(el, "url")),
		Title: textOf(findChild(el, "title")),
		Link:  textOf(findChild(el, "link")),
	}
	if w := textOf(findChild(el, "width")); w != "" {
		if n, err := strconv.Atoi(w); err == nil {
			img.Width = n
		}
	}
	if h := textOf(findChild(el, "height")); h != "" {
		if n, err := strconv.Atoi(h); err == nil {
			img.Height = n
		}
	}
	if d := textOf(findChild(el, "description")); d != "" {
		img.Description = d
	}
	return img
}

func parseRss2Cloud(el map[string]any) *Rss2Cloud {
	c := &Rss2Cloud{}
	if v, ok := attribute(el, "domain"); ok {
		c.Domain = v
	}
	if v, ok := attribute(el, "port"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			c.Port = n
		}
	}
	if v, ok := attribute(el, "path"); ok {
		c.Path = v
	}
	if v, ok := attribute(el, "registerProcedure"); ok {
		c.RegisterProcedure = v
	}
	if v, ok := attribute(el, "protocol"); ok {
		c.Protocol = v
	}
	return c
}

func parseRss2TextInput(el map[string]any) *Rss2TextInput {
	return &Rss2TextInput{
		Title:       textOf(findChild(el, "title")),
		Description: textOf(findChild(el, "description")),
		Name:        textOf(findChild(el, "name")),
		Link:        textOf(findChild(el, "link")),
	}
}

func parseRss2Item(el map[string]any) Rss2Item {
	item := Rss2Item{}
	if v := findChild(el, "title"); v != nil {
		item.Title = textOf(v)
	}
	if v := findChild(el, "link"); v != nil {
		item.Link = textOf(v)
	}
	if v := findChild(el, "description"); v != nil {
		item.Description = textOf(v)
	}
	if v := findChild(el, "author"); v != nil {
		item.Author = textOf(v)
	}
	if cs := findChildren(el, "category"); len(cs) > 0 {
		item.Categories = make([]Rss2Category, 0, len(cs))
		for _, c := range cs {
			cat := Rss2Category{Value: textOf(c)}
			if d, ok := attribute(c, "domain"); ok {
				cat.Domain = d
			}
			item.Categories = append(item.Categories, cat)
		}
	}
	if v := findChild(el, "comments"); v != nil {
		item.Comments = textOf(v)
	}
	if enc := findChild(el, "enclosure"); enc != nil {
		out := &Rss2Enclosure{}
		if v, ok := attribute(enc, "url"); ok {
			out.URL = v
		}
		if v, ok := attribute(enc, "length"); ok {
			if n, err := strconv.Atoi(v); err == nil {
				out.Length = n
			}
		}
		if v, ok := attribute(enc, "type"); ok {
			out.Type = v
		}
		item.Enclosure = out
	}
	if g := findChild(el, "guid"); g != nil {
		guid := &Rss2Guid{Value: textOf(g)}
		if v, ok := attribute(g, "isPermaLink"); ok {
			b := v != "false"
			guid.IsPermaLink = &b
		}
		item.Guid = guid
	}
	if v := findChild(el, "pubDate"); v != nil {
		item.PubDate = textOf(v)
	}
	if s := findChild(el, "source"); s != nil {
		src := &Rss2Source{Value: textOf(s)}
		if v, ok := attribute(s, "url"); ok {
			src.URL = v
		}
		item.Source = src
	}
	return item
}

func parseRss2(root map[string]any, version string) Rss2Feed {
	channel := findChild(root, "channel")
	if channel == nil {
		channel = root
	}
	v := "2.0"
	switch version {
	case "rss092":
		v = "0.92"
	case "rss091u", "rss091n":
		v = "0.91"
	}
	feed := Rss2Feed{
		Format:      "rss",
		Version:     v,
		Title:       textOf(findChild(channel, "title")),
		Link:        textOf(findChild(channel, "link")),
		Description: textOf(findChild(channel, "description")),
		Items:       []Rss2Item{},
	}
	if v := findChild(channel, "language"); v != nil {
		feed.Language = textOf(v)
	}
	if v := findChild(channel, "copyright"); v != nil {
		feed.Copyright = textOf(v)
	}
	if v := findChild(channel, "managingEditor"); v != nil {
		feed.ManagingEditor = textOf(v)
	}
	if v := findChild(channel, "webMaster"); v != nil {
		feed.WebMaster = textOf(v)
	}
	if v := findChild(channel, "pubDate"); v != nil {
		feed.PubDate = textOf(v)
	}
	if v := findChild(channel, "lastBuildDate"); v != nil {
		feed.LastBuildDate = textOf(v)
	}
	if cs := findChildren(channel, "category"); len(cs) > 0 {
		feed.Categories = make([]Rss2Category, 0, len(cs))
		for _, c := range cs {
			cat := Rss2Category{Value: textOf(c)}
			if d, ok := attribute(c, "domain"); ok {
				cat.Domain = d
			}
			feed.Categories = append(feed.Categories, cat)
		}
	}
	if v := findChild(channel, "generator"); v != nil {
		feed.Generator = textOf(v)
	}
	if v := findChild(channel, "docs"); v != nil {
		feed.Docs = textOf(v)
	}
	if c := findChild(channel, "cloud"); c != nil {
		feed.Cloud = parseRss2Cloud(c)
	}
	if t := findChild(channel, "ttl"); t != nil {
		if n, err := strconv.Atoi(textOf(t)); err == nil {
			feed.TTL = n
		}
	}
	if i := findChild(channel, "image"); i != nil {
		feed.Image = parseRss2Image(i)
	}
	ti := findChild(channel, "textInput")
	if ti == nil {
		ti = findChild(channel, "textinput")
	}
	if ti != nil {
		feed.TextInput = parseRss2TextInput(ti)
	}
	if sh := findChild(channel, "skipHours"); sh != nil {
		for _, h := range findChildren(sh, "hour") {
			if n, err := strconv.Atoi(textOf(h)); err == nil {
				feed.SkipHours = append(feed.SkipHours, n)
			}
		}
	}
	if sd := findChild(channel, "skipDays"); sd != nil {
		for _, d := range findChildren(sd, "day") {
			feed.SkipDays = append(feed.SkipDays, textOf(d))
		}
	}
	for _, item := range findChildren(channel, "item") {
		feed.Items = append(feed.Items, parseRss2Item(item))
	}
	return feed
}

// --- RSS 1.0 / 0.90 (RDF) parser ------------------------------------------

func parseRss1Image(el map[string]any) *Rss1Image {
	img := &Rss1Image{
		Title: textOf(findChild(el, "title")),
		Link:  textOf(findChild(el, "link")),
		URL:   textOf(findChild(el, "url")),
	}
	if v, ok := attribute(el, "rdf:about"); ok {
		img.About = v
	}
	return img
}

func parseRss1TextInput(el map[string]any) *Rss1TextInput {
	ti := &Rss1TextInput{
		Title:       textOf(findChild(el, "title")),
		Description: textOf(findChild(el, "description")),
		Name:        textOf(findChild(el, "name")),
		Link:        textOf(findChild(el, "link")),
	}
	if v, ok := attribute(el, "rdf:about"); ok {
		ti.About = v
	}
	return ti
}

func parseRss1Item(el map[string]any) Rss1Item {
	item := Rss1Item{
		Title: textOf(findChild(el, "title")),
		Link:  textOf(findChild(el, "link")),
	}
	if v, ok := attribute(el, "rdf:about"); ok {
		item.About = v
	}
	if d := findChild(el, "description"); d != nil {
		item.Description = textOf(d)
	}
	return item
}

func parseRss1(root map[string]any, version string) Rss1Feed {
	channel := findChild(root, "channel")
	feed := Rss1Feed{
		Format:  "rdf",
		Version: "1.0",
		Title:   textOf(findChild(channel, "title")),
		Link:    textOf(findChild(channel, "link")),
		Items:   []Rss1Item{},
	}
	if version == "rss090" {
		feed.Version = "0.90"
	}
	if channel != nil {
		if v, ok := attribute(channel, "rdf:about"); ok {
			feed.About = v
		}
		if d := findChild(channel, "description"); d != nil {
			feed.Description = textOf(d)
		}
	}
	if i := findChild(root, "image"); i != nil {
		feed.Image = parseRss1Image(i)
	}
	ti := findChild(root, "textinput")
	if ti == nil {
		ti = findChild(root, "textInput")
	}
	if ti != nil {
		feed.TextInput = parseRss1TextInput(ti)
	}
	for _, item := range findChildren(root, "item") {
		feed.Items = append(feed.Items, parseRss1Item(item))
	}
	return feed
}

// --- Native -> Atom conversions -------------------------------------------

var rssAuthorRe = regexp.MustCompile(`^\s*([^\s()]+@\S+)\s*(?:\(([^)]+)\))?\s*$`)

func parseRssPerson(s string) AtomPerson {
	m := rssAuthorRe.FindStringSubmatch(s)
	if m == nil {
		return AtomPerson{Name: strings.TrimSpace(s)}
	}
	name := m[2]
	if name == "" {
		name = m[1]
	}
	return AtomPerson{Name: name, Email: m[1]}
}

func rss2ToAtom(rss Rss2Feed) AtomFeed {
	out := AtomFeed{Format: "atom", Version: "1.0", Entries: []AtomEntry{}}
	if rss.Title != "" {
		out.Title = &AtomText{Type: "text", Value: rss.Title}
	}
	if rss.Description != "" {
		out.Subtitle = &AtomText{Type: "text", Value: rss.Description}
	}
	if rss.Copyright != "" {
		out.Rights = &AtomText{Type: "text", Value: rss.Copyright}
	}
	if rss.Link != "" {
		out.ID = rss.Link
	}
	if rss.LastBuildDate != "" {
		out.Updated = rss.LastBuildDate
	} else if rss.PubDate != "" {
		out.Updated = rss.PubDate
	}
	if rss.Generator != "" {
		out.Generator = &AtomGenerator{Value: rss.Generator}
	}
	if rss.Image != nil && rss.Image.URL != "" {
		out.Logo = rss.Image.URL
	}
	if rss.Link != "" {
		out.Links = []AtomLink{{Href: rss.Link, Rel: "alternate"}}
	}
	if len(rss.Categories) > 0 {
		out.Categories = make([]AtomCategory, 0, len(rss.Categories))
		for _, c := range rss.Categories {
			out.Categories = append(out.Categories, AtomCategory{Term: c.Value, Scheme: c.Domain})
		}
	}
	if rss.ManagingEditor != "" || rss.WebMaster != "" {
		me := rss.ManagingEditor
		if me == "" {
			me = rss.WebMaster
		}
		out.Authors = []AtomPerson{parseRssPerson(me)}
	}
	for _, item := range rss.Items {
		out.Entries = append(out.Entries, rss2ItemToAtomEntry(item))
	}
	return out
}

func rss2ItemToAtomEntry(item Rss2Item) AtomEntry {
	entry := AtomEntry{}
	if item.Title != "" {
		entry.Title = &AtomText{Type: "text", Value: item.Title}
	}
	if item.Description != "" {
		entry.Summary = &AtomText{Type: "html", Value: item.Description}
	}
	if item.PubDate != "" {
		entry.Published = item.PubDate
		entry.Updated = item.PubDate
	}
	if item.Guid != nil {
		entry.ID = item.Guid.Value
	} else if item.Link != "" {
		entry.ID = item.Link
	}
	links := []AtomLink{}
	if item.Link != "" {
		links = append(links, AtomLink{Href: item.Link, Rel: "alternate"})
	}
	if item.Enclosure != nil && item.Enclosure.URL != "" {
		l := AtomLink{Href: item.Enclosure.URL, Rel: "enclosure"}
		if item.Enclosure.Type != "" {
			l.Type = item.Enclosure.Type
		}
		if item.Enclosure.Length > 0 {
			l.Length = item.Enclosure.Length
		}
		links = append(links, l)
	}
	if item.Comments != "" {
		links = append(links, AtomLink{Href: item.Comments, Rel: "replies", Type: "text/html"})
	}
	if len(links) > 0 {
		entry.Links = links
	}
	if item.Author != "" {
		entry.Authors = []AtomPerson{parseRssPerson(item.Author)}
	}
	if len(item.Categories) > 0 {
		entry.Categories = make([]AtomCategory, 0, len(item.Categories))
		for _, c := range item.Categories {
			entry.Categories = append(entry.Categories, AtomCategory{Term: c.Value, Scheme: c.Domain})
		}
	}
	if item.Source != nil {
		src := &AtomEntrySource{Format: "atom", Version: "1.0"}
		if item.Source.Value != "" {
			src.Title = &AtomText{Type: "text", Value: item.Source.Value}
		}
		if item.Source.URL != "" {
			src.Links = []AtomLink{{Href: item.Source.URL, Rel: "self"}}
		}
		entry.Source = src
	}
	return entry
}

func rss1ToAtom(rss Rss1Feed) AtomFeed {
	out := AtomFeed{Format: "atom", Version: "1.0", Entries: []AtomEntry{}}
	if rss.Title != "" {
		out.Title = &AtomText{Type: "text", Value: rss.Title}
	}
	if rss.Description != "" {
		out.Subtitle = &AtomText{Type: "text", Value: rss.Description}
	}
	switch {
	case rss.About != "":
		out.ID = rss.About
	case rss.Link != "":
		out.ID = rss.Link
	}
	if rss.Link != "" {
		out.Links = []AtomLink{{Href: rss.Link, Rel: "alternate"}}
	}
	if rss.Image != nil && rss.Image.URL != "" {
		out.Logo = rss.Image.URL
	}
	for _, it := range rss.Items {
		e := AtomEntry{}
		if it.Title != "" {
			e.Title = &AtomText{Type: "text", Value: it.Title}
		}
		if it.Description != "" {
			e.Summary = &AtomText{Type: "text", Value: it.Description}
		}
		switch {
		case it.About != "":
			e.ID = it.About
		case it.Link != "":
			e.ID = it.Link
		}
		if it.Link != "" {
			e.Links = []AtomLink{{Href: it.Link, Rel: "alternate"}}
		}
		out.Entries = append(out.Entries, e)
	}
	return out
}

// --- Top-level convert ----------------------------------------------------

// Convert turns a parsed XML element tree (as produced by
// github.com/tabnas/xml/go) into the requested feed shape.
func Convert(root any, format string) (any, error) {
	if format == "raw" {
		return root, nil
	}
	el, ok := asElement(root)
	if !ok {
		return nil, fmt.Errorf("feed: input did not parse to an XML element")
	}
	d := Detect(el)
	if d.Dialect == "unknown" {
		return nil, fmt.Errorf(
			"feed: unrecognized root element %q; expected one of feed, rss, RDF",
			localName(el),
		)
	}

	var native any
	switch d.Dialect {
	case "atom":
		native = parseAtom(el, d.Version == "atom03")
	case "rss":
		native = parseRss2(el, d.Version)
	case "rdf":
		native = parseRss1(el, d.Version)
	}

	if format == "native" {
		return native, nil
	}

	// Atom (default).
	switch n := native.(type) {
	case AtomFeed:
		return n, nil
	case Rss2Feed:
		return rss2ToAtom(n), nil
	case Rss1Feed:
		return rss1ToAtom(n), nil
	}
	return nil, fmt.Errorf("feed: internal: unhandled native type")
}

// --- jsonic Plugin --------------------------------------------------------

// Feed is the Jsonic plugin entry point. Register via:
//
//	j := jsonic.Make()
//	j.UseDefaults(feed.Feed, feed.Defaults)
//	result, err := j.Parse(rssOrAtomSource)
//
// The result is an AtomFeed (default), an Rss2Feed / Rss1Feed
// (when "format" is "native"), or the raw map[string]any XML element
// tree (when "format" is "raw").
func Feed(j *jsonic.Jsonic, options map[string]any) error {
	if j.Decoration("feed-init") != nil {
		return nil
	}
	j.Decorate("feed-init", true)

	if err := j.UseDefaults(xml.Xml, xml.Defaults); err != nil {
		return fmt.Errorf("feed: setup xml: %w", err)
	}

	format, _ := options["format"].(string)
	if format == "" {
		format = "atom"
	}

	// The xml rule's bc fires once per close, including extra times when
	// `r: xml` recurses to consume trailing whitespace. Mirror @xml-bc's
	// own guard: only run when an element was actually parsed in *this*
	// iteration (r.Child.Node is set), so the conversion happens exactly
	// once on the same iteration that @xml-bc copied the element to root.
	j.Rule("xml", func(rs *jsonic.RuleSpec, _ *jsonic.Parser) {
		rs.AddBC(func(r *jsonic.Rule, ctx *jsonic.Context) {
			if r.Child == nil || r.Child == jsonic.NoRule || r.Child.Node == nil {
				return
			}
			if r.Node == nil {
				return
			}
			out, err := Convert(r.Node, format)
			if err != nil {
				ctx.ParseErr = &jsonic.Token{
					Name: "#BD", Tin: jsonic.TinBD,
					Err: err.Error(), Why: err.Error(), Src: err.Error(),
				}
				return
			}
			r.Node = out
			if ctx.Root != nil {
				ctx.Root.Node = out
			}
		})
	})

	return nil
}
