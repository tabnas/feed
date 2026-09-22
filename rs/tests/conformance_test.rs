// Conformance against the two FETCHED third-party corpora, in Rust.
//
//   rubys/feedvalidator  — the suite behind the W3C Feed Validation
//                          Service, fetched by scripts/fetch-feedvalidator.sh
//   kurtmckee/feedparser — fetched by scripts/fetch-feedparser.sh
//
// This file is the Rust half of `ts/test/feedvalidator.test.ts`,
// `ts/test/feedparser-conformance.test.ts` and `go/conformance_test.go`.
// The three runtimes classify and assert identically, so a divergence
// shows up as one of them going red rather than as three baselines
// drifting apart. Until this file existed the conformance numbers in
// `../AGENTS.md` were a TypeScript and Go claim only; they are a claim
// about all three ports now.
//
// Neither corpus is committed. Each is fetched at a pinned commit into a
// gitignored directory, and when one is absent these tests FAIL LOUDLY
// after trying to fetch it. Nothing here skips: a conformance suite that
// quietly does not run is worse than no suite at all, which is the rule
// `../AGENTS.md` states under "No test may silently not-run".

mod common;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::path::{Path, PathBuf};
use std::process::Command;

use regex::Regex;
use tabnas::{Tabnas, Value};
use tabnas_feed::{detect, FeedFormat, FeedOptions};

use common::repo_root;

// --- corpus plumbing ------------------------------------------------------

/// The corpus directory, fetched first when it is not there. It NEVER
/// returns without one: a missing corpus panics with the command that
/// fixes it. `cargo test` has no pretest hook, which is how a suite ends
/// up silently never running.
fn require_corpus(dir: &str, script: &str) -> PathBuf {
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
fn walk_xml(dir: &Path) -> Vec<PathBuf> {
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
fn read_file(path: &Path) -> String {
    let bytes = fs::read(path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    String::from_utf8_lossy(&bytes).into_owned()
}

/// The path as the failure lists and the enumerated sets spell it:
/// relative to the repository root, forward slashes.
fn rel_path(path: &Path) -> String {
    path.strip_prefix(repo_root())
        .unwrap_or(path)
        .to_string_lossy()
        .replace('\\', "/")
}

/// The document element read from the SOURCE TEXT, deliberately
/// independent of the parser under test, so classification cannot be
/// biased by the thing being measured.
fn root_local_name(src: &str) -> String {
    let comment = Regex::new(r"(?s)<!--.*?-->").expect("a valid pattern");
    let pi = Regex::new(r"(?s)<\?.*?\?>").expect("a valid pattern");
    let doctype = Regex::new(r"(?is)<!DOCTYPE.*?>").expect("a valid pattern");
    let root = Regex::new(r"<\s*([A-Za-z_][-A-Za-z0-9_:.]*)").expect("a valid pattern");

    let stripped = comment.replace_all(src, "");
    let stripped = pi.replace_all(&stripped, "");
    let stripped = doctype.replace_all(&stripped, "");
    let Some(found) = root.captures(&stripped) else {
        return String::new();
    };
    let name = found[1].to_string();
    match name.rfind(':') {
        Some(at) => name[at + 1..].to_string(),
        None => name,
    }
}

/// The document elements this crate claims: RSS 0.90 to 2.0, Atom 0.3
/// and 1.0.
fn is_feed_root(src: &str) -> bool {
    matches!(root_local_name(src).as_str(), "feed" | "rss" | "RDF")
}

/// A failure list, capped, because a broken change fails thousands of
/// files and the first forty say the same thing as all of them.
fn fmt_fails(fails: &[String]) -> String {
    let limit = 40;
    let mut out = String::new();
    for (index, fail) in fails.iter().enumerate() {
        if index == limit {
            out.push_str(&format!("  ... and {} more\n", fails.len() - limit));
            break;
        }
        out.push_str(&format!("  {fail}\n"));
    }
    out
}

/// The documented stack: jsonic, then this plugin.
fn conform_parser(format: FeedFormat) -> Tabnas {
    common::parser_with(&FeedOptions {
        format,
        ..FeedOptions::default()
    })
}

/// A parse that reports a panic as a rejection rather than taking the
/// whole suite down, mirroring `safeParse` in the Go twin. The engine
/// reports a malformed document through `Err`; a panic would be a defect
/// in this crate, and it is recorded per file instead of losing every
/// other file's result.
fn safe_parse(parser: &Tabnas, src: &str) -> Result<Value, String> {
    // The engine's error type is large, so it is reduced to its first
    // rendered line inside the closure rather than carried out of it:
    // `clippy::result_large_err` is denied here and the message is all a
    // failure list wants anyway.
    let parsed = catch_unwind(AssertUnwindSafe(|| {
        parser.parse(src).map_err(|error| {
            common::strip_ansi(&error.to_string())
                .lines()
                .next()
                .unwrap_or_default()
                .to_string()
        })
    }));
    match parsed {
        Ok(Ok(value)) => Ok(value),
        Ok(Err(message)) => Err(message),
        Err(panic) => Err(format!(
            "PANIC: {}",
            panic
                .downcast_ref::<String>()
                .cloned()
                .or_else(|| panic.downcast_ref::<&str>().map(|s| s.to_string()))
                .unwrap_or_else(|| "non-string panic payload".to_string())
        )),
    }
}

/// True when a parse result is the normalised Atom shape.
fn is_atom_shaped(value: &Value) -> bool {
    value.to_json().get("format").and_then(|f| f.as_str()) == Some("atom")
}

// --- feedvalidator --------------------------------------------------------

/// Four corpus documents that are objectively NOT well-formed but carry
/// an `Expect:` naming a validator-level diagnostic instead of
/// `SAXError`: upstream annotates the thing the case is ABOUT, and a real
/// SAX parse of any of them fails first. They move into the must-REJECT
/// bucket, and they are not excused: the violation is quoted from the
/// file, and the set is asserted to be exactly this list, so a fifth
/// cannot be added silently and a repaired one cannot stay listed. Kept
/// identical to `NOT_WELL_FORMED` in the TypeScript twin and
/// `notWellFormed` in the Go one.
fn not_well_formed() -> BTreeMap<&'static str, &'static str> {
    BTreeMap::from([
        // <copyright type="application/xhtml+xml"> ... </rights>
        (
            "atom/must/feed_copyright_is_inline.xml",
            "XML 1.0 3 WFC \"Element Type Match\": <copyright> is closed by </rights>",
        ),
        // <copyright type="text/html" mode="xml"> ... </rights>
        (
            "atom/must/feed_copyright_is_inline_2.xml",
            "XML 1.0 3 WFC \"Element Type Match\": <copyright> is closed by </rights>",
        ),
        // <sx:sharing .../> is self-closing, then a stray </sx:sharing>
        // follows, so the end tag lands on the still-open <feed>.
        (
            "ext/feedsync/sharing_until_rfc822.xml",
            "XML 1.0 3 WFC \"Element Type Match\": stray </sx:sharing> after a self-closed <sx:sharing/>",
        ),
        // <invalid:tag xmlns:bogus="tag:foo.bar"/> declares `bogus` and
        // uses `invalid`. The Expect is about the bogus namespace URI,
        // but the undeclared prefix is a hard error first.
        (
            "atom/6.1/invalid-namespace.xml",
            "Namespaces in XML 1.0 NSC \"Prefix Declared\": prefix `invalid` is never declared",
        ),
    ])
}

const TESTCASES_PREFIX: &str = "test/feedvalidator/testcases/";

#[test]
fn feedvalidator_conformance() {
    let suite = require_corpus("feedvalidator", "fetch-feedvalidator.sh");
    let files = walk_xml(&suite.join("testcases"));
    assert!(
        1000 < files.len(),
        "feedvalidator corpus looks truncated: {} .xml files under {}; \
         re-run ./scripts/fetch-feedvalidator.sh",
        files.len(),
        suite.display()
    );

    let reclassified = not_well_formed();
    let sax_expect = Regex::new(r"Expect:[^\n]*SAXError").expect("a valid pattern");

    let mut sax: Vec<PathBuf> = Vec::new();
    let mut feeds: Vec<PathBuf> = Vec::new();
    let mut out_of_claim: Vec<PathBuf> = Vec::new();
    let mut seen_reclassified: BTreeSet<&str> = BTreeSet::new();

    for path in &files {
        let src = read_file(path);
        let key = rel_path(path)
            .strip_prefix(TESTCASES_PREFIX)
            .unwrap_or_default()
            .to_string();
        if let Some((listed, _)) = reclassified.get_key_value(key.as_str()) {
            seen_reclassified.insert(listed);
            sax.push(path.clone());
        } else if sax_expect.is_match(&src) {
            sax.push(path.clone());
        } else if is_feed_root(&src) {
            feeds.push(path.clone());
        } else {
            out_of_claim.push(path.clone());
        }
    }

    // Guard the reclassification: every listed path must exist in the
    // fetched corpus, so a rename upstream is a red test rather than a
    // silent exemption.
    let missing: Vec<String> = reclassified
        .keys()
        .filter(|key| !seen_reclassified.contains(*key))
        .map(|key| (*key).to_string())
        .collect();
    assert!(
        missing.is_empty(),
        "not_well_formed lists paths that are not in the fetched corpus: {missing:?}"
    );

    let parser = conform_parser(FeedFormat::Atom);

    // 1. Not well-formed, so it must be rejected.
    assert!(!sax.is_empty(), "no SAXError cases found — corpus wrong?");
    let mut fails: Vec<String> = Vec::new();
    for path in &sax {
        if safe_parse(&parser, &read_file(path)).is_ok() {
            fails.push(format!("ACCEPTED but not well-formed: {}", rel_path(path)));
        }
    }
    println!(
        "feedvalidator invalid: {}/{} rejected ({} annotated \"Expect: SAXError\" + {} reclassified)",
        sax.len() - fails.len(),
        sax.len(),
        sax.len() - seen_reclassified.len(),
        seen_reclassified.len()
    );
    assert!(
        fails.is_empty(),
        "must-reject failures ({}/{}):\n{}",
        fails.len(),
        sax.len(),
        fmt_fails(&fails)
    );

    // 2. Well-formed RSS or Atom, so it must be accepted, as the Atom
    //    shape rather than as merely "something".
    let mut fails: Vec<String> = Vec::new();
    for path in &feeds {
        match safe_parse(&parser, &read_file(path)) {
            Err(error) => fails.push(format!("{}: {error}", rel_path(path))),
            Ok(value) if !is_atom_shaped(&value) => {
                fails.push(format!("{}: no atom-shaped result", rel_path(path)));
            }
            Ok(_) => {}
        }
    }
    let roots: BTreeSet<String> = out_of_claim
        .iter()
        .map(|path| root_local_name(&read_file(path)))
        .collect();
    println!(
        "feedvalidator valid: {}/{} accepted (+{} documents outside the RSS/Atom claim, \
         not asserted: {})",
        feeds.len() - fails.len(),
        feeds.len(),
        out_of_claim.len(),
        roots.into_iter().collect::<Vec<_>>().join(", ")
    );
    assert!(
        fails.is_empty(),
        "must-accept failures ({}/{}):\n{}",
        fails.len(),
        feeds.len(),
        fmt_fails(&fails)
    );

    // 3. Accepting is only half a value check: the dialect must be right
    //    too, and the corpus directory is the oracle.
    let raw = conform_parser(FeedFormat::Raw);
    let mut fails: Vec<String> = Vec::new();
    let mut checked = 0usize;
    for path in &feeds {
        let relative = rel_path(path);
        let want = if relative.starts_with(&format!("{TESTCASES_PREFIX}atom/")) {
            "atom"
        } else if relative.starts_with(&format!("{TESTCASES_PREFIX}rss20/")) {
            "rss"
        } else {
            continue;
        };
        checked += 1;
        match safe_parse(&raw, &read_file(path)) {
            Err(error) => fails.push(format!("{relative}: {error}")),
            Ok(root) => {
                let got = detect(&root);
                if got.dialect.as_str() != want {
                    fails.push(format!(
                        "{relative}: dialect={}/{}, want {want}",
                        got.dialect.as_str(),
                        got.version.as_str()
                    ));
                }
            }
        }
    }
    println!(
        "feedvalidator detect: {}/{} correct dialect",
        checked - fails.len(),
        checked
    );
    // An empty failure list is also what nothing-checked looks like. The
    // want is keyed off the corpus path, so a path-shape change upstream
    // would zero `checked` and pass this while asserting nothing.
    assert!(
        1000 < checked,
        "only {checked} documents classified by directory"
    );
    assert!(
        fails.is_empty(),
        "dialect-detection failures ({}/{}):\n{}",
        fails.len(),
        checked,
        fmt_fails(&fails)
    );
}
