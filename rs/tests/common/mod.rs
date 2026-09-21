// Shared test helpers. Cargo compiles this module into EVERY integration
// test binary, so an item only one binary uses is dead code in the
// others; the allow keeps that from being a warning rather than hiding
// anything real.
#![allow(dead_code)]

use std::path::{Path, PathBuf};

use tabnas::{Tabnas, Value};
use tabnas_feed::{detect, FeedFormat, FeedOptions};
use tabnas_support::{find_spec_dir, Failure, Runner};

/// The repository root: the parent of `rs/`.
pub fn repo_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("rs/ has a parent")
        .to_path_buf()
}

/// The shared `test/spec` directory, found by walking up from the crate
/// rather than by counting `..` hops.
pub fn spec_dir() -> PathBuf {
    find_spec_dir(Some(Path::new(env!("CARGO_MANIFEST_DIR"))))
        .expect("a test/spec directory above rs/")
}

/// The vendored well-formed corpus. It is COMMITTED, so it can never
/// legitimately be absent: every caller fails rather than skips.
pub fn wellformed_dir() -> PathBuf {
    repo_root().join("test").join("feedparser-wellformed")
}

/// An engine value as the fixture data model, through JSON: the
/// `jsonFlatten` of the other two runners. It drops a key whose value is
/// `undefined`, which is what the canonical feed model sets an absent
/// field to, and collapses class identity and field order.
pub fn to_value(value: &Value) -> tabnas_support::Value {
    tabnas_support::Value::from(value.to_json())
}

/// A parse error as the runner's failure. The feed layer's own
/// rejections are prose, so the rendered report is what a fixture pins.
pub fn to_failure(error: tabnas::TabnasError) -> Failure {
    Failure::new(error.code.clone())
        .at(error.row, error.col)
        .with_message(error.to_string())
}

/// A parser built the way the canonical suites build one: jsonic, then
/// the feed plugin with the row's options.
pub fn parser_with(options: &FeedOptions) -> Tabnas {
    let mut parser = tabnas_jsonic::make();
    parser
        .use_plugin(tabnas_feed::plugin(), Some(options.to_value()))
        .expect("the feed plugin installs on a jsonic instance");
    parser
}

/// The parser a test that does not care about options gets.
pub fn default_parser() -> Tabnas {
    parser_with(&FeedOptions::default())
}

/// The raw-tree parser, for detection over an untouched element tree.
pub fn raw_parser() -> Tabnas {
    parser_with(&FeedOptions {
        format: FeedFormat::Raw,
        ..FeedOptions::default()
    })
}

/// The SGR colour sequences the engine writes into rendered error
/// messages, removed so a fixture's expected text can stay plain.
pub fn strip_ansi(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    let mut chars = text.chars().peekable();
    while let Some(ch) = chars.next() {
        if ch == '\u{1b}' && chars.peek() == Some(&'[') {
            chars.next();
            for next in chars.by_ref() {
                if next == 'm' {
                    break;
                }
            }
            continue;
        }
        out.push(ch);
    }
    out
}

/// The JSON text of an engine value, for short assertions.
pub fn json(value: &Value) -> String {
    value.to_json().to_string()
}

/// The fixture runner both the parity suite and the divergence register
/// drive, so a row means the same thing in either file.
///
/// `mode` is the fixture's SECOND COLUMN HEADER, which says what the file
/// asserts: `expected` is the parsed feed, `detect` is the dialect report
/// for the input. That is per file, which is why there is a runner per
/// file rather than one over the directory.
pub fn spec_runner(mode: &str) -> Runner {
    let detect_mode = mode == "detect";
    Runner::new_with_row(move |input, row| {
        let raw = row.named("opts");
        // Detection is asserted over the RAW parse, so those fixtures pin
        // the dialect rather than passing their own options.
        let options = if detect_mode {
            FeedOptions {
                format: FeedFormat::Raw,
                ..FeedOptions::default()
            }
        } else if raw.trim().is_empty() {
            FeedOptions::default()
        } else {
            let bag: serde_json::Value = serde_json::from_str(raw)
                .map_err(|error| Failure::message(format!("opts column is not JSON: {error}")))?;
            FeedOptions::from_value(&Value::from_json(&bag))
        };

        let parsed = parser_with(&options).parse(input).map_err(to_failure)?;
        Ok(to_value(&if detect_mode {
            detect(&parsed).to_value()
        } else {
            parsed
        }))
    })
    // feed's `ERROR:<want>` cells hold a fragment of the MESSAGE --
    // `unrecognized root element "kml"`, `character data is not allowed
    // outside the root element` -- rather than an error code. These
    // rejections come from the feed layer's own validation, which reports
    // what is wrong in prose rather than through a code the engine
    // assigns. A bare `ERROR` still accepts any failure.
    .match_error(|failure, want, _row| strip_ansi(&failure.message).contains(want))
    .input("input")
    .expected(mode)
}
