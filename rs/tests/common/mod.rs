// Shared test helpers. Cargo compiles this module into EVERY integration
// test binary, so an item only one binary uses is dead code in the
// others; the allow keeps that from being a warning rather than hiding
// anything real.
#![allow(dead_code)]

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

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

/// The corpus directory, fetched first when it is not there. It NEVER
/// returns without one: a missing corpus panics with the command that
/// fixes it. `cargo test` has no pretest hook, which is how a suite ends
/// up silently never running.
pub fn require_corpus(dir: &str, script: &str) -> PathBuf {
    let root = repo_root();
    let full = root.join("test").join(dir);
    if !full.exists() {
        let fetch = Command::new("node")
            .arg(root.join("scripts").join("fetch-corpus.mjs"))
            .arg(dir)
            .status();
        if let Err(error) = fetch {
            eprintln!("auto-fetch of the {dir} corpus failed: {error}");
        }
    }
    assert!(
        full.exists(),
        "\n\nCONFORMANCE CORPUS MISSING: {}\n\
         This suite cannot run without it, and must never be skipped.\n\
         Fetch it (pinned commit, idempotent):\n\n    ./scripts/{script}\n\n\
         See scripts/{script} for the upstream URL and pinned SHA.\n",
        full.display()
    );
    full
}

/// Every `.xml` under a directory, sorted, so a failure list is stable
/// between runs and between runtimes.
pub fn walk_xml(dir: &Path) -> Vec<PathBuf> {
    let mut out = Vec::new();
    let mut stack = vec![dir.to_path_buf()];
    while let Some(next) = stack.pop() {
        let entries =
            fs::read_dir(&next).unwrap_or_else(|e| panic!("read {}: {e}", next.display()));
        for entry in entries {
            let entry = entry.expect("a directory entry");
            let path = entry.path();
            if path.is_dir() {
                stack.push(path);
            } else if path.extension().is_some_and(|ext| ext == "xml") {
                out.push(path);
            }
        }
    }
    out.sort();
    out
}

/// A corpus file as a string, LOSSILY. Some feedvalidator cases are not
/// valid UTF-8 on purpose (a declared encoding the bytes contradict), and
/// the other two runtimes see them decoded rather than rejected: Node's
/// `readFileSync(p, 'utf8')` substitutes U+FFFD, and Go's `string(b)`
/// yields U+FFFD for each invalid byte as the parser walks it. Reading
/// strictly here would make this port disagree with both over a
/// difference in file IO rather than in parsing.
pub fn read_file(path: &Path) -> String {
    let bytes = fs::read(path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    String::from_utf8_lossy(&bytes).into_owned()
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
