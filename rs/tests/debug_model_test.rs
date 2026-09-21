// Composition: this plugin layered with the official debug plugin,
// mirroring ts/test/debug-model.test.ts.
//
// `tabnas-debug` is a declared dev-dependency, so it is always present
// here and this test can never skip. The canonical suite used to skip
// when it could not resolve the plugin, which silently removed the whole
// composition suite while the run still reported green; it now fails
// instead, and so does this.
//
// What it asserts is the fact that shapes this whole crate: the feed
// plugin adds NO rules. It installs the xml grammar and hooks the
// existing `xml` rule's before-close, so the structured model reports the
// XML rule set and an `xml` start rule, with nothing feed-named in it.

mod common;

use tabnas_debug::{model, DebugOptions};

use common::default_parser;

fn debug_parser() -> tabnas::Tabnas {
    let mut parser = default_parser();
    tabnas_debug::apply(&mut parser, DebugOptions::quiet()).expect("the debug plugin installs");
    parser
}

#[test]
fn the_parser_still_works_with_the_debug_plugin_installed() {
    let value = debug_parser()
        .parse("<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>Hi</title></feed>")
        .expect("the document parses");
    let feed = value.to_json();
    assert_eq!(feed["format"], "atom");
    assert_eq!(feed["title"]["value"], "Hi");
}

#[test]
fn the_model_reports_the_xml_grammar_and_nothing_feed_named() {
    let parser = debug_parser();
    let described = model(&parser);

    let mut names: Vec<&str> = described
        .rules
        .iter()
        .map(|rule| rule.name.as_str())
        .collect();
    names.sort_unstable();
    assert_eq!(names, ["child", "content", "element", "xml"]);

    assert_eq!(described.config.start, "xml");

    // The plugin stack lists this plugin and the xml plugin it pulls in.
    for wanted in ["feed", "xml"] {
        assert!(
            described.plugins.iter().any(|plugin| plugin.name == wanted),
            "the plugin list has no {wanted:?}: {:?}",
            described
                .plugins
                .iter()
                .map(|plugin| plugin.name.as_str())
                .collect::<Vec<_>>()
        );
    }

    // Structural facts of the xml grammar: the start rule opens an
    // element, and elements recurse into elements through content and
    // child.
    let edges = |name: &str| {
        described
            .graph
            .iter()
            .find(|edges| edges.name == name)
            .unwrap_or_else(|| panic!("the model has no {name} rule"))
            .open_push
            .clone()
    };
    assert!(edges("xml").contains(&"element".to_string()));
    assert!(edges("child").contains(&"element".to_string()));
}

#[test]
fn the_model_is_json_serialisable_and_round_trips() {
    let parser = debug_parser();
    let described = model(&parser);
    let rendered = serde_json::to_value(&described.rules).expect("the rules serialise");
    let again: serde_json::Value =
        serde_json::from_str(&rendered.to_string()).expect("the rules round-trip");
    assert_eq!(rendered, again);
}
