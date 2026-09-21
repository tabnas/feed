// In-language cases, mirrored from ts/test/feed.test.ts,
// ts/test/feedparser.test.ts and go/feed_test.go, plus the cases that are
// specific to this port: the JavaScript character classes and the
// integer reader it has to reproduce, and the bounds on untrusted input.
//
// The dialect-by-dialect expectations live in test/spec/*.tsv and are run
// by tests/parity_test.rs, in all three runtimes.

mod common;

use std::fs;
use std::path::{Path, PathBuf};

use tabnas_feed::{
    convert, detect, js_parse_int, js_trim, FeedDialect, FeedFormat, FeedOptions, FeedVersion,
};

use common::{default_parser, json, parser_with, raw_parser, wellformed_dir};

fn native_parser() -> tabnas::Tabnas {
    parser_with(&FeedOptions {
        format: FeedFormat::Native,
        ..FeedOptions::default()
    })
}

// --- errors ---------------------------------------------------------------

#[test]
fn an_unrecognized_root_element_is_an_error() {
    let error = default_parser()
        .parse("<not-a-feed/>")
        .expect_err("an unrecognized root is refused");
    assert!(
        error.to_string().contains("unrecognized root"),
        "got {error}"
    );
}

#[test]
fn the_raw_format_never_reaches_the_root_check() {
    let root = raw_parser()
        .parse("<not-a-feed/>")
        .expect("raw output is the tree, whatever the root is");
    assert_eq!(root.to_json()["localName"], "not-a-feed");
}

// --- raw format -----------------------------------------------------------

#[test]
fn raw_returns_the_underlying_element_tree() {
    let root = raw_parser()
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>Hi</title></feed>")
        .expect("the document parses");
    let tree = root.to_json();
    assert_eq!(tree["localName"], "feed");
    assert_eq!(tree["namespace"], "http://www.w3.org/2005/Atom");
    assert!(tree["children"].is_array());
}

#[test]
fn detect_on_raw_output_identifies_the_dialect() {
    let root = raw_parser()
        .parse("<rss version=\"2.0\"><channel><title>x</title></channel></rss>")
        .expect("the document parses");
    assert_eq!(
        detect(&root),
        tabnas_feed::Detection {
            dialect: FeedDialect::Rss,
            version: FeedVersion::Rss20,
        }
    );
}

#[test]
fn detect_reports_unknown_for_anything_that_is_not_an_element() {
    for value in [
        tabnas::Value::Undefined,
        tabnas::Value::Null,
        tabnas::Value::String("feed".to_string()),
        tabnas::Value::array(Vec::new()),
    ] {
        assert_eq!(detect(&value).dialect, FeedDialect::Unknown);
        assert_eq!(detect(&value).version, FeedVersion::Unknown);
    }
}

// --- plugin form ----------------------------------------------------------

#[test]
fn the_default_format_is_atom() {
    let value = default_parser()
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"/>")
        .expect("the document parses");
    assert_eq!(
        json(&value),
        r#"{"format":"atom","version":"1.0","entries":[]}"#
    );
}

#[test]
fn native_preserves_the_dialect_shape() {
    let value = native_parser()
        .parse("<rss version=\"2.0\"><channel><title>x</title></channel></rss>")
        .expect("the document parses");
    let native = value.to_json();
    assert_eq!(native["format"], "rss");
    assert_eq!(native["version"], "2.0");
}

#[test]
fn trailing_white_space_does_not_convert_twice() {
    // The xml rule's before-close fires more than once here, because the
    // rule recurses to consume the trailing white space. The plugin must
    // not re-convert what it already converted.
    let value = default_parser()
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>Hi</title></feed>\n  \n")
        .expect("the document parses");
    let feed = value.to_json();
    assert_eq!(feed["format"], "atom");
    assert_eq!(feed["title"]["value"], "Hi");
}

#[test]
fn installing_the_plugin_twice_is_a_no_op() {
    let mut parser = tabnas_jsonic::make();
    parser
        .use_plugin(tabnas_feed::plugin(), None)
        .expect("the first install succeeds");
    parser
        .use_plugin(tabnas_feed::plugin(), None)
        .expect("the second install is refused quietly");
    let value = parser
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>Hi</title></feed>")
        .expect("the document parses");
    assert_eq!(value.to_json()["title"]["value"], "Hi");
}

#[test]
fn the_plugin_contributes_no_rules_of_its_own() {
    let mut names = tabnas_feed::make().rule_names();
    names.sort();
    assert_eq!(names, ["child", "content", "element", "xml"]);
}

#[test]
fn strict_namespaces_is_on_by_default_and_can_be_turned_off() {
    let src = "<feed xmlns=\"http://www.w3.org/2005/Atom\"><dc:language>en</dc:language></feed>";
    assert!(
        default_parser().parse(src).is_err(),
        "an unbound prefix is an error by default"
    );
    let lenient = parser_with(&FeedOptions {
        strict_namespaces: false,
        ..FeedOptions::default()
    });
    assert!(
        lenient.parse(src).is_ok(),
        "strictNamespaces: false opts back into bare XML 1.0"
    );
}

// --- options --------------------------------------------------------------

#[test]
fn an_option_bag_reads_the_way_the_canonical_plugin_reads_one() {
    let read = |text: &str| {
        let bag: serde_json::Value = serde_json::from_str(text).expect("the bag is JSON");
        FeedOptions::from_value(&tabnas::Value::from_json(&bag))
    };
    assert_eq!(read("{}"), FeedOptions::default());
    assert_eq!(read(r#"{"format":"native"}"#).format, FeedFormat::Native);
    assert_eq!(read(r#"{"format":"raw"}"#).format, FeedFormat::Raw);
    // Only an explicit false turns namespace checking off.
    assert!(read(r#"{"strictNamespaces":true}"#).strict_namespaces);
    assert!(!read(r#"{"strictNamespaces":false}"#).strict_namespaces);
    assert!(read(r#"{"strictNamespaces":"no"}"#).strict_namespaces);
    // An unrecognised format name is the default, because the canonical
    // `convert` tests for `raw` and `native` and falls through.
    assert_eq!(read(r#"{"format":"bogus"}"#).format, FeedFormat::Atom);
}

#[test]
fn the_option_bag_round_trips() {
    for options in [
        FeedOptions::default(),
        FeedOptions {
            format: FeedFormat::Native,
            strict_namespaces: false,
        },
        FeedOptions {
            format: FeedFormat::Raw,
            strict_namespaces: true,
        },
    ] {
        assert_eq!(FeedOptions::from_value(&options.to_value()), options);
    }
}

// --- the questions a dialect table has to answer --------------------------

/// A namespace the plugin does not know is not an error and not a
/// dialect: an element is matched by LOCAL NAME throughout, so a
/// declared foreign namespace simply contributes nothing.
#[test]
fn an_unknown_namespace_is_carried_rather_than_refused() {
    let value = default_parser()
        .parse(
            "<feed xmlns=\"http://www.w3.org/2005/Atom\" \
             xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\
             <dc:language>en</dc:language><title>x</title></feed>",
        )
        .expect("a declared foreign namespace is well-formed");
    assert_eq!(
        json(&value),
        r#"{"format":"atom","version":"1.0","entries":[],"title":{"type":"text","value":"x"}}"#
    );

    // The same is true of the ROOT: only the local name selects the
    // dialect, so an Atom-shaped document in a namespace of its own is
    // still read as Atom 1.0.
    let root = raw_parser()
        .parse("<feed xmlns=\"http://example.com/not-atom\"/>")
        .expect("the document parses");
    assert_eq!(detect(&root).version, FeedVersion::Atom10);
}

/// A mixed-dialect document is read as whatever its ROOT says, and the
/// elements of the other dialect are matched by local name like any
/// other. `<atom:link>` inside an RSS channel is therefore the channel's
/// `link`, which carries no text, so the mapping produces an empty one.
#[test]
fn a_mixed_dialect_document_follows_its_root() {
    let value = default_parser()
        .parse(
            "<rss version=\"2.0\" xmlns:atom=\"http://www.w3.org/2005/Atom\">\
             <channel><title>t</title>\
             <atom:link href=\"http://a/\" rel=\"self\"/>\
             <item><title>i</title></item></channel></rss>",
        )
        .expect("the document parses");
    assert_eq!(
        json(&value),
        r#"{"format":"atom","version":"1.0","entries":[{"title":{"type":"text","value":"i"}}],"title":{"type":"text","value":"t"}}"#
    );
}

/// A document that DECLARES one dialect and is written in another is read
/// as the one it declares, and the foreign body simply does not match:
/// detection never looks past the root element.
#[test]
fn a_declared_dialect_wins_over_the_body() {
    let rss_saying_atom = native_parser()
        .parse("<rss version=\"2.0\"><entry><title>i</title></entry></rss>")
        .expect("the document parses");
    assert_eq!(
        json(&rss_saying_atom),
        r#"{"format":"rss","version":"2.0","title":"","link":"","description":"","items":[]}"#
    );

    let atom_saying_rss = native_parser()
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"><channel><title>t</title></channel></feed>")
        .expect("the document parses");
    assert_eq!(
        json(&atom_saying_rss),
        r#"{"format":"atom","version":"1.0","entries":[]}"#
    );
}

#[test]
fn an_rss_root_with_no_version_is_read_as_two_point_zero() {
    let root = raw_parser()
        .parse("<rss><channel><title>t</title></channel></rss>")
        .expect("the document parses");
    assert_eq!(detect(&root).version, FeedVersion::Rss20);
}

#[test]
fn every_declared_rss_version_lands_where_the_table_says() {
    for (declared, want) in [
        ("2.0", FeedVersion::Rss20),
        ("2.0.1", FeedVersion::Rss20),
        ("2.0.2", FeedVersion::Rss20),
        ("0.92", FeedVersion::Rss092),
        ("0.93", FeedVersion::Rss092),
        ("0.94", FeedVersion::Rss092),
        ("0.91", FeedVersion::Rss091u),
        ("9.9", FeedVersion::Rss20),
    ] {
        let src = format!("<rss version=\"{declared}\"><channel/></rss>");
        let root = raw_parser().parse(&src).expect("the document parses");
        assert_eq!(detect(&root).version, want, "version=\"{declared}\"");
    }
}

/// The Netscape 0.91 variant is in the version vocabulary and is never
/// reported: the two 0.91 dialects differ only by DOCTYPE, and detection
/// assumes the Userland one. Pinned so that changing it is deliberate.
#[test]
fn the_netscape_zero_nine_one_variant_is_never_reported() {
    let src = "<!DOCTYPE rss SYSTEM \"http://my.netscape.com/publish/formats/rss-0.91.dtd\">\
               <rss version=\"0.91\"><channel/></rss>";
    let root = raw_parser().parse(src).expect("the document parses");
    assert_eq!(detect(&root).version, FeedVersion::Rss091u);
}

#[test]
fn an_rdf_document_without_a_channel_is_rss_one_point_zero() {
    let root = raw_parser()
        .parse(
            "<rdf:RDF xmlns:rdf=\"http://www.w3.org/1999/02/22-rdf-syntax-ns#\" \
             xmlns=\"http://purl.org/rss/1.0/\"><item><title>i</title></item></rdf:RDF>",
        )
        .expect("the document parses");
    assert_eq!(detect(&root).version, FeedVersion::Rss10);
}

// --- the JavaScript classes this port reproduces --------------------------

/// `String.prototype.trim` is not `str::trim`. ECMA-262 white space has
/// U+FEFF and has not U+0085; the Unicode `White_Space` property that
/// Rust uses is the reverse, and both characters reach this crate as
/// ordinary element content.
#[test]
fn trimming_uses_the_javascript_white_space_class() {
    assert_eq!(js_trim("\u{feff}x\u{feff}"), "x");
    assert_ne!(
        js_trim("\u{feff}x\u{feff}"),
        "\u{feff}x\u{feff}".trim(),
        "str::trim does not remove a byte-order mark"
    );
    assert_eq!(js_trim("\u{85}x\u{85}"), "\u{85}x\u{85}");
    assert_ne!(
        js_trim("\u{85}x\u{85}"),
        "\u{85}x\u{85}".trim(),
        "str::trim does remove a next-line control"
    );
    for space in [
        '\t', '\n', '\u{b}', '\u{c}', '\r', ' ', '\u{a0}', '\u{1680}', '\u{2000}', '\u{200a}',
        '\u{2028}', '\u{2029}', '\u{202f}', '\u{205f}', '\u{3000}', '\u{feff}',
    ] {
        assert_eq!(
            js_trim(&format!("{space}x{space}")),
            "x",
            "U+{:04X}",
            space as u32
        );
    }
}

#[test]
fn element_text_is_trimmed_the_javascript_way() {
    let value = default_parser()
        .parse(
            "<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>\u{feff}Hi\u{feff}</title></feed>",
        )
        .expect("the document parses");
    assert_eq!(value.to_json()["title"]["value"], "Hi");

    let value = default_parser()
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>\u{85}Hi\u{85}</title></feed>")
        .expect("the document parses");
    assert_eq!(value.to_json()["title"]["value"], "\u{85}Hi\u{85}");
}

/// The author pattern is a ported JavaScript regex, so its `\s` and `\S`
/// are written out rather than left to the `regex` crate, which would
/// read them as the Unicode property and disagree on the same two
/// characters.
#[test]
fn the_author_pattern_uses_the_javascript_classes() {
    let person = |text: &str| {
        let src = format!(
            "<rss version=\"2.0\"><channel><item><author>{text}</author></item></channel></rss>"
        );
        json(&default_parser().parse(&src).expect("the document parses"))
    };
    assert!(person("j@example.com (Jane Doe)")
        .contains(r#"{"name":"Jane Doe","email":"j@example.com"}"#));
    assert!(person("j@example.com").contains(r#"{"name":"j@example.com","email":"j@example.com"}"#));
    // A byte-order mark is white space to the pattern, so the address
    // still matches; the Unicode class would not admit it.
    assert!(person("\u{feff}j@example.com\u{feff}")
        .contains(r#"{"name":"j@example.com","email":"j@example.com"}"#));
    // Anything the pattern does not match is a bare name.
    assert!(person("Jane Doe").contains(r#"{"name":"Jane Doe"}"#));
}

/// `parseInt(text, 10)` is neither `str::parse` nor Go's `strconv.Atoi`.
#[test]
fn integers_are_read_the_javascript_way() {
    assert_eq!(js_parse_int("1337"), 1337.0);
    assert_eq!(js_parse_int("12abc"), 12.0);
    assert_eq!(js_parse_int("  42  "), 42.0);
    assert_eq!(js_parse_int("+5"), 5.0);
    assert_eq!(js_parse_int("-3"), -3.0);
    // Radix ten, so the scan stops at the `x`.
    assert_eq!(js_parse_int("0x10"), 0.0);
    assert!(js_parse_int("abc").is_nan());
    assert!(js_parse_int("").is_nan());
    assert!(js_parse_int("-").is_nan());
    // A byte-order mark is leading white space, a next-line control is
    // not: the same class as everywhere else.
    assert_eq!(js_parse_int("\u{feff}7"), 7.0);
    assert!(js_parse_int("\u{85}7").is_nan());
    // Longer than 2^53: the nearest double, as the canonical reader
    // produces.
    assert_eq!(js_parse_int("9007199254740993"), 9007199254740992.0);
}

/// A value that is not a number at all becomes a NaN in the parsed feed
/// and a `null` once the result is rendered as JSON, which is what
/// `JSON.stringify` does with the canonical plugin's NaN.
#[test]
fn an_unreadable_length_becomes_null_in_the_rendered_result() {
    let value = default_parser()
        .parse(
            "<feed xmlns=\"http://www.w3.org/2005/Atom\"><entry>\
             <link href=\"h\" length=\"abc\"/></entry></feed>",
        )
        .expect("the document parses");
    assert!(
        json(&value).contains(r#""length":null"#),
        "got {}",
        json(&value)
    );

    let value = default_parser()
        .parse(
            "<feed xmlns=\"http://www.w3.org/2005/Atom\"><entry>\
             <link href=\"h\" length=\"12abc\"/></entry></feed>",
        )
        .expect("the document parses");
    assert_eq!(value.to_json()["entries"][0]["links"][0]["length"], 12.0);
}

// --- convert, called directly ---------------------------------------------

#[test]
fn convert_runs_over_a_tree_the_xml_crate_produced() {
    let root = tabnas_xml::parse("<rss version=\"2.0\"><channel><title>x</title></channel></rss>")
        .expect("the document parses");
    assert_eq!(
        json(&convert(&root, FeedFormat::Native).expect("an rss root converts")),
        r#"{"format":"rss","version":"2.0","title":"x","link":"","description":"","items":[]}"#
    );
    assert_eq!(
        json(&convert(&root, FeedFormat::Raw).expect("raw is the tree")),
        json(&root)
    );
    let bad = tabnas_xml::parse("<nope/>").expect("the document parses");
    assert!(convert(&bad, FeedFormat::Atom)
        .expect_err("an unrecognized root is refused")
        .contains("unrecognized root element \"nope\""));
    assert!(convert(&tabnas::Value::Null, FeedFormat::Atom)
        .expect_err("a non-element is refused")
        .contains("did not parse to an XML element"));
}

// --- the vendored well-formed corpus --------------------------------------

/// The corpus is COMMITTED, so it can never legitimately be absent:
/// this fails rather than skips, as `requireWellformed` does in Go and
/// `loadDir` does in TypeScript. A runner that reports green having run
/// nothing is indistinguishable from coverage that was never there.
fn corpus_files(sub: &str) -> Vec<PathBuf> {
    let dir = wellformed_dir().join(sub);
    let mut out: Vec<PathBuf> = fs::read_dir(&dir)
        .unwrap_or_else(|error| {
            panic!(
                "the vendored corpus is missing at {}: {error}",
                dir.display()
            )
        })
        .filter_map(|entry| entry.ok())
        .map(|entry| entry.path())
        .filter(|path| path.extension().is_some_and(|ext| ext == "xml"))
        .collect();
    out.sort();
    assert!(
        !out.is_empty(),
        "no .xml files under {}: the vendored corpus is truncated",
        dir.display()
    );
    out
}

fn corpus_expect() -> Vec<(&'static str, FeedDialect, Vec<FeedVersion>)> {
    vec![
        ("atom10", FeedDialect::Atom, vec![FeedVersion::Atom10]),
        ("atom", FeedDialect::Atom, vec![FeedVersion::Atom03]),
        (
            "rss",
            FeedDialect::Rss,
            vec![
                FeedVersion::Rss20,
                FeedVersion::Rss092,
                FeedVersion::Rss091u,
                FeedVersion::Rss091n,
            ],
        ),
        (
            "rdf",
            FeedDialect::Rdf,
            vec![FeedVersion::Rss10, FeedVersion::Rss090],
        ),
    ]
}

fn base(path: &Path) -> String {
    path.file_name()
        .unwrap_or_default()
        .to_string_lossy()
        .into_owned()
}

#[test]
fn the_corpus_detects_as_its_directory_says() {
    let parser = raw_parser();
    let mut failures = Vec::new();
    for (sub, dialect, versions) in corpus_expect() {
        for path in corpus_files(sub) {
            let src = fs::read_to_string(&path).expect("a corpus file is readable");
            let root = match parser.parse(&src) {
                Ok(root) => root,
                Err(error) => {
                    failures.push(format!("{sub}/{}: parse error {error}", base(&path)));
                    continue;
                }
            };
            let got = detect(&root);
            if got.dialect != dialect {
                failures.push(format!("{sub}/{}: dialect={:?}", base(&path), got.dialect));
            } else if !versions.contains(&got.version) {
                failures.push(format!("{sub}/{}: version={:?}", base(&path), got.version));
            }
        }
    }
    assert!(
        failures.is_empty(),
        "detection failures:\n  {}",
        failures.join("\n  ")
    );
}

#[test]
fn every_corpus_document_parses_to_an_atom_shape() {
    let parser = default_parser();
    let mut failures = Vec::new();
    for (sub, _, _) in corpus_expect() {
        for path in corpus_files(sub) {
            let src = fs::read_to_string(&path).expect("a corpus file is readable");
            match parser.parse(&src) {
                Ok(value) => {
                    if value.to_json()["format"] != "atom" {
                        failures.push(format!("{sub}/{}: bad shape", base(&path)));
                    }
                }
                Err(error) => failures.push(format!("{sub}/{}: {error}", base(&path))),
            }
        }
    }
    assert!(
        failures.is_empty(),
        "parse failures:\n  {}",
        failures.join("\n  ")
    );
}

/// The targeted value checks of go/feed_test.go `TestCorpusTargets`, in
/// the same order. "It did not throw" is not a conformance result.
#[test]
fn the_corpus_carries_the_values_it_is_supposed_to() {
    let parser = default_parser();
    let value_of = |relative: &str| {
        let path = wellformed_dir().join(relative);
        let src =
            fs::read_to_string(&path).unwrap_or_else(|error| panic!("{}: {error}", path.display()));
        parser
            .parse(&src)
            .unwrap_or_else(|error| panic!("{relative}: {error}"))
            .to_json()
    };

    assert_eq!(
        value_of("atom10/entry_title.xml")["entries"][0]["title"]["value"],
        "Example Atom"
    );
    assert_eq!(
        value_of("atom10/entry_author_email.xml")["entries"][0]["authors"][0]["email"],
        "me@example.com"
    );
    assert_eq!(
        value_of("atom10/entry_author_name.xml")["entries"][0]["authors"][0]["name"],
        "Example author"
    );
    assert!(value_of("atom10/entry_id.xml")["entries"][0]["id"].is_string());
    assert!(value_of("atom10/entry_link_href.xml")["entries"][0]["links"][0]["href"].is_string());
    assert!(value_of("atom/entry_title.xml")["entries"][0]["title"]["value"] != "");
    assert!(value_of("atom/entry_issued.xml")["entries"][0]["published"].is_string());
    assert!(value_of("atom/entry_modified.xml")["entries"][0]["updated"].is_string());
    assert_eq!(
        value_of("rss/channel_title.xml")["title"]["value"],
        "Example feed"
    );
    assert_eq!(
        value_of("rss/item_title.xml")["entries"][0]["title"]["value"],
        "Item 1 title"
    );
    assert!(value_of("rss/item_link.xml")["entries"][0]["links"][0]["href"].is_string());
    assert!(value_of("rss/item_guid.xml")["entries"][0]["id"].is_string());
    let enclosure = value_of("rss/item_enclosure_url.xml");
    assert!(
        enclosure["entries"][0]["links"]
            .as_array()
            .expect("the entry has links")
            .iter()
            .any(|link| link["rel"] == "enclosure"),
        "no enclosure link in {enclosure}"
    );
    assert_eq!(
        value_of("rdf/rdf_channel_title.xml")["title"]["value"],
        "Example feed"
    );
    assert_eq!(
        value_of("rdf/rdf_item_title.xml")["entries"][0]["title"]["value"],
        "Example title"
    );
    assert_eq!(
        value_of("rdf/rss090_channel_title.xml")["title"]["value"],
        "Example title"
    );
    assert_eq!(
        value_of("rdf/rdf_item_rdf_about.xml")["entries"][0]["id"],
        "http://example.org/1"
    );
}

// --- untrusted input ------------------------------------------------------

/// Deeply nested, very long, unterminated, empty and control-character
/// input must not panic, hang or overflow the stack. Every one of these
/// has to return, whether with a value or with an error.
///
/// The depth here is `WALKABLE_DEPTH`, and it is deliberately modest for
/// two reasons. Nesting costs super-linear time in the XML layer below
/// this crate, measured on a release build of the same stack: 100 levels
/// in 2.6 ms, 800 in 20 ms, 1,600 in 67 ms and 3,200 in 415 ms. And past
/// the bound recorded on `WALKABLE_DEPTH` the process ABORTS rather than
/// returning, so a deeper case here would not assert more, it would kill
/// the runner. That curve, the abort, and the recursion in
/// `Value::to_json` and in dropping a `Value` are owned by `tabnas-xml`
/// and the engine; this crate adds no depth of its own and sets no
/// budget of its own. See `AGENTS.md`.
#[test]
fn hostile_input_returns_rather_than_aborting() {
    let parser = default_parser();

    let deep = format!(
        "<feed xmlns=\"http://www.w3.org/2005/Atom\">{}{}</feed>",
        "<a>".repeat(WALKABLE_DEPTH),
        "</a>".repeat(WALKABLE_DEPTH)
    );
    let _ = parser.parse(&deep);

    let long = format!(
        "<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>{}</title></feed>",
        "x".repeat(50_000)
    );
    assert_eq!(
        parser
            .parse(&long)
            .expect("a long but well-formed document parses")
            .to_json()["title"]["value"]
            .as_str()
            .map(str::len),
        Some(50_000)
    );

    for hostile in [
        "",
        "<feed",
        "<feed xmlns=\"http://www.w3.org/2005/Atom\">",
        "<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>x",
        "<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>\u{0}</title></feed>",
        "<\u{fffd}",
        "&&&&&&&&",
        "<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>\u{feff}\u{85}\u{2028}</title></feed>",
    ] {
        let _ = parser.parse(hostile);
    }
}

/// The deepest nesting a caller may walk, and the stack it is walked on.
///
/// This crate sets no depth budget, so the bound is a measured property
/// of the stack below it rather than a refusal it can rely on: on a
/// release build and an 8 MiB stack, `Value::to_json` over a `raw` parse
/// overflows and ABORTS the process at about 8,300 levels, and the
/// default `atom` format aborts inside `parse` at about 20,000. An abort
/// is not a `Result`, so the test cannot assert where it happens; what
/// it can assert is the range the documentation promises. Raising this
/// constant past the measured bound would not make the test stronger, it
/// would make the runner abort. `AGENTS.md` and `README.md` carry the
/// numbers and tell a caller to cap its input.
const WALKABLE_DEPTH: usize = 512;
const WALKABLE_STACK: usize = 8 * 1024 * 1024;

/// A deep RAW tree is handed back as an engine value, and both `to_json`
/// and dropping one recurse. What this pins is the promise the
/// documentation makes and no more: up to `WALKABLE_DEPTH`, on a stack
/// of `WALKABLE_STACK`, a tree the parser accepts is one the caller can
/// walk and drop. The stack size is set here rather than inherited from
/// the harness, because the whole property is a stack measurement and a
/// harness default is not part of the contract.
#[test]
fn a_deep_tree_that_parses_can_be_walked_and_dropped() {
    std::thread::Builder::new()
        .stack_size(WALKABLE_STACK)
        .spawn(|| {
            let parser = raw_parser();
            let deep = format!(
                "{}{}",
                "<a>".repeat(WALKABLE_DEPTH),
                "</a>".repeat(WALKABLE_DEPTH)
            );
            match parser.parse(&deep) {
                Err(error) => assert!(!error.code.is_empty(), "a refusal carries a code"),
                Ok(value) => {
                    let rendered = value.to_json();
                    assert!(rendered.is_object());
                    drop(value);
                }
            }
        })
        .expect("the walker thread spawns")
        .join()
        .expect("the walker thread finishes");
}
