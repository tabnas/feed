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

// --- feedparser -----------------------------------------------------------
//
// Twin of `ts/test/feedparser-conformance.test.ts` and
// `TestFeedParserConformance` in `go/conformance_test.go`. The two
// enumerated sets below and the two value floors are kept identical to
// those twins, so a runtime divergence shows up as one side going red
// rather than as three baselines drifting apart.

// Measured on main at 7d2103f (2026-08-09): 375 of 1360 machine-checkable
// upstream `Expect:` assertions hold. This is a RATCHET: raise both
// numbers when the parser improves, never lower either to get green. The
// denominator is floored too, so dropping value checks cannot be used to
// improve the ratio.
const FEEDPARSER_VALUE_CORRECT_FLOOR: usize = 375;
const FEEDPARSER_VALUE_CHECKED_FLOOR: usize = 1360;

const FEEDPARSER_WF_PREFIX: &str = "test/feedparser/wellformed/";
const FEEDPARSER_ILL_PREFIX: &str = "test/feedparser/illformed/";

/// Upstream's `illformed/` directory is keyed off feedparser's `bozo`
/// flag, which is much broader than XML well-formedness. These documents
/// are ACCEPTED by this crate and each is listed with the reason, so the
/// set is a statement rather than an excuse. It is asserted to be EXACTLY
/// this list: a newly-accepted ill-formed document is red, and so is a
/// listed document that starts being rejected, whose entry must then be
/// deleted.
fn feedparser_accepted_illformed() -> BTreeMap<&'static str, &'static str> {
    BTreeMap::from([
        // Well-formed XML carrying an invalid DOCTYPE. Upstream itself
        // annotates this `Expect: not bozo and feed['title'] == 'found'`,
        // so accepting it is the CORRECT behaviour; it sits in illformed/
        // for a different reason.
        (
            "always_strip_doctype.xml",
            "well-formed; upstream Expect is `not bozo`, so accepting is correct",
        ),
        // Declared-versus-actual character encoding mismatches. Detecting
        // these needs the raw byte stream and a charset detector; this
        // crate is handed an already-decoded string, so the evidence is
        // gone before it is called.
        (
            "chardet/big5.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/eucjp.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/euckr.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/gb2312.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/koi8r.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/shiftjis.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/tis620.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        (
            "chardet/windows1255.xml",
            "encoding detection: needs raw bytes, not a decoded string",
        ),
        // GeoRSS and GML coordinate errors. Well-formed XML; the defect is
        // in the meaning of an extension element this crate does not
        // model.
        (
            "geo/georss_point_no_coords.xml",
            "GeoRSS semantics, not XML well-formedness",
        ),
        (
            "geo/georss_polygon_insufficient_coords.xml",
            "GeoRSS semantics, not XML well-formedness",
        ),
        (
            "geo/gml_point.xml",
            "GML semantics, not XML well-formedness",
        ),
        // Well-formed iso-8859-7 document; upstream records that the
        // non-ASCII date crashed its own date parser. This crate does not
        // parse dates.
        (
            "http_high_bit_date.xml",
            "upstream records a date-parser crash, not a well-formedness defect",
        ),
    ])
}

/// Five documents where [`detect`] disagrees with the upstream
/// annotation, each recorded with what this port currently reports, so
/// the set is exact in both directions.
fn feedparser_version_known_wrong() -> BTreeMap<&'static str, &'static str> {
    BTreeMap::from([
        // RSS 0.90 is an RDF document; detect reports the RDF-era RSS 1.0.
        ("rss/rss_version_090.xml", "rss10"),
        // Netscape and Userland 0.91 are told apart by the DOCTYPE, which
        // detect does not read.
        ("rss/rss_version_091_netscape.xml", "rss091u"),
        // 0.93 and 0.94 are not modelled; both collapse onto 0.92.
        ("rss/rss_version_093.xml", "rss092"),
        ("rss/rss_version_094.xml", "rss092"),
        // <rss> with no version attribute: upstream reports the bare 'rss'.
        ("rss/rss_version_missing.xml", "rss20"),
    ])
}

// --- the `Expect:` evaluator ----------------------------------------------
//
// Mirror of `ts/test/expect-eval.ts` and the Go twin. The supported form
// is `not bozo and <path> == '<string>'`, with any number of `and`-joined
// clauses. Everything else (time tuples, `len()`, `has_key()`, dict
// literals, bare truthiness) is unsupported and COUNTED, never silently
// passed.

/// One step of an upstream accessor path: a property name or an array
/// index.
#[derive(Debug, Clone, PartialEq, Eq)]
enum Step {
    Key(String),
    Index(usize),
}

impl std::fmt::Display for Step {
    fn fmt(&self, out: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Step::Key(key) => write!(out, "{key}"),
            Step::Index(index) => write!(out, "{index}"),
        }
    }
}

struct ExpectClause {
    path: String,
    steps: Vec<Step>,
    want: String,
}

/// The `Expect:` clauses of a corpus file, or `None` when the annotation
/// is absent or in a form this evaluator does not support. `None` is
/// counted by the caller, never treated as a pass.
fn parse_expect(src: &str) -> Option<Vec<ExpectClause>> {
    let expect = Regex::new(r"Expect:[ \t]*(.*)").expect("a valid pattern");
    let clause = Regex::new(
        r"^((?:feed|entries\[\d+\])(?:\[(?:'[^']*'|\d+)\])*)\s*==\s*'((?:[^'\\]|\\.)*)'$",
    )
    .expect("a valid pattern");
    let head = Regex::new(r"^(?:feed|entries\[(\d+)\])").expect("a valid pattern");
    let step = Regex::new(r"\[(?:'([^']*)'|(\d+))\]").expect("a valid pattern");
    let paren = Regex::new(r"^\((.*)\)$").expect("a valid pattern");
    let and = Regex::new(r"\s+and\s+").expect("a valid pattern");

    let found = expect.captures(src)?;
    let text = found[1].trim();
    let rest = text.strip_prefix("not bozo and ")?.trim();

    let mut out = Vec::new();
    for part in and.split(rest) {
        let part = part.trim();
        let unwrapped = paren
            .captures(part)
            .map(|inner| inner[1].trim().to_string());
        let part = unwrapped.as_deref().unwrap_or(part);
        let matched = clause.captures(part)?;
        let path = matched[1].to_string();

        let mut steps = Vec::new();
        let head_match = head.captures(&path).expect("the clause pattern implies it");
        match head_match.get(1) {
            None => steps.push(Step::Key("feed".to_string())),
            Some(index) => {
                steps.push(Step::Key("entries".to_string()));
                steps.push(Step::Index(index.as_str().parse().unwrap_or(0)));
            }
        }
        let tail = &path[head_match[0].len()..];
        for found in step.captures_iter(tail) {
            match (found.get(1), found.get(2)) {
                (_, Some(index)) => steps.push(Step::Index(index.as_str().parse().unwrap_or(0))),
                (Some(key), None) => steps.push(Step::Key(key.as_str().to_string())),
                (None, None) => {}
            }
        }

        let want = matched[2]
            .replace("\\'", "'")
            .replace("\\\"", "\"")
            .replace("\\\\", "\\");
        out.push(ExpectClause { path, steps, want });
    }
    if out.is_empty() {
        return None;
    }
    Some(out)
}

fn feed_text(key: &str) -> Option<&'static str> {
    match key {
        "title" => Some("title"),
        "subtitle" | "tagline" | "description" | "info" => Some("subtitle"),
        "rights" | "copyright" => Some("rights"),
        _ => None,
    }
}

fn entry_text(key: &str) -> Option<&'static str> {
    match key {
        "title" => Some("title"),
        "summary" | "description" => Some("summary"),
        "rights" | "copyright" => Some("rights"),
        _ => None,
    }
}

fn person_key(key: &str) -> Option<&'static str> {
    match key {
        "name" => Some("name"),
        "email" => Some("email"),
        "href" | "url" | "uri" => Some("uri"),
        _ => None,
    }
}

/// feedparser reports text-construct types as MIME types; the Atom shape
/// keeps RFC 4287's `text` / `html` / `xhtml` tokens.
fn mime_of(token: &str) -> Option<&'static str> {
    match token {
        "text" => Some("text/plain"),
        "html" => Some("text/html"),
        "xhtml" => Some("application/xhtml+xml"),
        _ => None,
    }
}

type Json = serde_json::Value;

fn mget(value: &Json, key: &str) -> Json {
    value.get(key).cloned().unwrap_or(Json::Null)
}

fn aget(value: &Json, index: usize) -> Json {
    value
        .as_array()
        .and_then(|items| items.get(index))
        .cloned()
        .unwrap_or(Json::Null)
}

fn first_alternate(links: &Json) -> Json {
    let Some(items) = links.as_array() else {
        return Json::Null;
    };
    for link in items {
        let rel = mget(link, "rel");
        let rel = rel.as_str().unwrap_or_default();
        if rel.is_empty() || rel == "alternate" {
            return mget(link, "href");
        }
    }
    Json::Null
}

fn join_steps(steps: &[Step]) -> String {
    steps
        .iter()
        .map(Step::to_string)
        .collect::<Vec<_>>()
        .join(".")
}

/// A text construct: bare is its `value`, `['value']` the same, and
/// `['type']` the MIME spelling of its token.
fn text_at(obj: &Json, prop: &str, tail: &[Step]) -> Result<Json, String> {
    let text = mget(obj, prop);
    if tail.is_empty() {
        return Ok(mget(&text, "value"));
    }
    if tail.len() == 1 {
        if let Step::Key(key) = &tail[0] {
            if key == "value" {
                return Ok(mget(&text, "value"));
            }
            if key == "type" {
                let token = mget(&text, "type");
                return Ok(match token.as_str().and_then(mime_of) {
                    Some(mime) => Json::String(mime.to_string()),
                    None => Json::Null,
                });
            }
        }
    }
    Err(format!("detail.{}", join_steps(tail)))
}

/// Resolve an upstream accessor path against the Atom shape. `Ok` is a
/// mapped path, whose value may legitimately be `null`; `Err` carries WHY
/// the path has no mapping, which the caller tallies and prints.
fn resolve_expect(feed: &Json, steps: &[Step]) -> Result<Json, String> {
    let is_entry = steps.first() == Some(&Step::Key("entries".to_string()));
    let (obj, path) = if is_entry {
        let entries = mget(feed, "entries");
        let Step::Index(index) = steps[1] else {
            return Err("no such entry".to_string());
        };
        let entry = aget(&entries, index);
        if entry.is_null() {
            return Err("no such entry".to_string());
        }
        (entry, &steps[2..])
    } else {
        (feed.clone(), &steps[1..])
    };

    if path.is_empty() {
        return Err("whole-object comparison".to_string());
    }
    let Step::Key(key) = &path[0] else {
        return Err("index-at-root".to_string());
    };
    let tail = &path[1..];

    let text = if is_entry { entry_text } else { feed_text };
    if let Some(prop) = text(key) {
        return text_at(&obj, prop, tail);
    }
    if let Some(stem) = key.strip_suffix("_detail") {
        if let Some(prop) = text(stem) {
            return text_at(&obj, prop, tail);
        }
    }

    let index_at = |at: usize| match tail.get(at) {
        Some(Step::Index(index)) => Some(*index),
        _ => None,
    };
    let name_at = |at: usize| match tail.get(at) {
        Some(Step::Key(key)) => key.as_str(),
        _ => "",
    };

    match (key.as_str(), tail.len()) {
        ("id" | "guid", 0) => return Ok(mget(&obj, "id")),
        ("updated", 0) => return Ok(mget(&obj, "updated")),
        ("published", 0) => return Ok(mget(&obj, "published")),
        ("link", 0) => return Ok(first_alternate(&mget(&obj, "links"))),
        ("links", 2) => {
            if let Some(index) = index_at(0) {
                if matches!(name_at(1), "href" | "rel" | "type" | "title") {
                    return Ok(mget(&aget(&mget(&obj, "links"), index), name_at(1)));
                }
            }
        }
        ("author_detail", 1) => {
            if let Some(prop) = person_key(name_at(0)) {
                return Ok(mget(&aget(&mget(&obj, "authors"), 0), prop));
            }
        }
        ("authors" | "contributors", 2) => {
            if let (Some(index), Some(prop)) = (index_at(0), person_key(name_at(1))) {
                return Ok(mget(&aget(&mget(&obj, key), index), prop));
            }
        }
        ("tags", 2) => {
            if let Some(index) = index_at(0) {
                if matches!(name_at(1), "term" | "scheme" | "label") {
                    return Ok(mget(&aget(&mget(&obj, "categories"), index), name_at(1)));
                }
            }
        }
        ("generator", 0) => return Ok(mget(&mget(&obj, "generator"), "value")),
        ("content", 2) => {
            if let Some(index) = index_at(0) {
                if matches!(name_at(1), "value" | "type") {
                    if index != 0 {
                        return Err("content[n>0]".to_string());
                    }
                    let content = mget(&obj, "content");
                    if name_at(1) == "type" {
                        let token = mget(&content, "type");
                        return Ok(match token.as_str().and_then(mime_of) {
                            Some(mime) => Json::String(mime.to_string()),
                            None => token,
                        });
                    }
                    return Ok(mget(&content, "value"));
                }
            }
        }
        ("image", 1) if name_at(0) == "href" => return Ok(mget(&obj, "logo")),
        ("source", _) if !tail.is_empty() => {
            let source = mget(&obj, "source");
            if source.is_null() {
                return Ok(Json::Null);
            }
            let mut nested = vec![Step::Key("feed".to_string())];
            nested.extend_from_slice(tail);
            return resolve_expect(&source, &nested);
        }
        _ => {}
    }

    let prefix = if is_entry { "entry." } else { "feed." };
    Err(format!("{prefix}{}", join_steps(path)))
}

/// The n most frequent keys of a tally, ties broken by key, so the line
/// is stable between runs and between runtimes.
fn top_counts(counts: &BTreeMap<String, usize>, n: usize) -> String {
    let mut all: Vec<(&String, &usize)> = counts.iter().collect();
    all.sort_by(|left, right| right.1.cmp(left.1).then(left.0.cmp(right.0)));
    all.truncate(n);
    all.iter()
        .map(|(key, count)| format!("{key}({count})"))
        .collect::<Vec<_>>()
        .join(" ")
}

#[test]
fn feedparser_conformance() {
    let suite = require_corpus("feedparser", "fetch-feedparser.sh");
    let wf_all = walk_xml(&suite.join("wellformed"));
    let ill_all = walk_xml(&suite.join("illformed"));
    // Floors on the corpus itself: a truncated fetch would otherwise
    // shrink every denominator below and turn this green while measuring
    // almost nothing.
    assert!(
        1000 < wf_all.len(),
        "feedparser wellformed corpus looks truncated: {} files; \
         re-run ./scripts/fetch-feedparser.sh",
        wf_all.len()
    );
    assert!(
        !ill_all.is_empty(),
        "feedparser illformed corpus missing; re-run ./scripts/fetch-feedparser.sh"
    );

    // Documents whose root is not feed/rss/RDF are outside the README's
    // claim (RSS 0.90 to 2.0, Atom 0.3 and 1.0): counted and printed,
    // never asserted.
    let wf: Vec<PathBuf> = wf_all
        .iter()
        .filter(|path| is_feed_root(&read_file(path)))
        .cloned()
        .collect();
    let out_of_claim = wf_all.len() - wf.len();

    let parser = conform_parser(FeedFormat::Atom);

    // 1. Well-formed, so it must parse to the Atom shape.
    let mut fails: Vec<String> = Vec::new();
    for path in &wf {
        match safe_parse(&parser, &read_file(path)) {
            Err(error) => fails.push(format!("{}: {error}", rel_path(path))),
            Ok(value) if !is_atom_shaped(&value) => {
                fails.push(format!("{}: not an atom-shaped result", rel_path(path)));
            }
            Ok(_) => {}
        }
    }
    println!(
        "feedparser wellformed: {}/{} parsed (+{} non-RSS/Atom roots, outside the claim, \
         not asserted)",
        wf.len() - fails.len(),
        wf.len(),
        out_of_claim
    );
    assert!(
        fails.is_empty(),
        "parse failures ({}/{}):\n{}",
        fails.len(),
        wf.len(),
        fmt_fails(&fails)
    );

    // 2. The value-level ratchet: upstream's own `Expect:` assertions.
    let mut fails: Vec<String> = Vec::new();
    let mut unmapped: BTreeMap<String, usize> = BTreeMap::new();
    let (mut supported, mut correct, mut unmapped_files) = (0usize, 0usize, 0usize);
    for path in &wf {
        let src = read_file(path);
        let Some(clauses) = parse_expect(&src) else {
            continue;
        };
        supported += 1;
        let parsed = match safe_parse(&parser, &src) {
            Err(error) => {
                fails.push(format!("{}: parse threw: {error}", rel_path(path)));
                continue;
            }
            Ok(value) => value,
        };
        let shape = parsed.to_json();

        let mut saw_unmapped = false;
        let mut bad = String::new();
        for clause in &clauses {
            match resolve_expect(&shape, &clause.steps) {
                Err(why) => {
                    saw_unmapped = true;
                    *unmapped.entry(why).or_insert(0) += 1;
                }
                Ok(value) => {
                    if value.as_str() != Some(clause.want.as_str()) {
                        bad = format!(
                            "{} = {}, expected {:?}",
                            clause.path,
                            serde_json::to_string(&value)
                                .unwrap_or_else(|_| "<unmarshalable>".to_string()),
                            clause.want
                        );
                        break;
                    }
                }
            }
        }
        if !bad.is_empty() {
            fails.push(format!("{}: {bad}", rel_path(path)));
        } else if saw_unmapped {
            unmapped_files += 1;
        } else {
            correct += 1;
        }
    }
    let checked = correct + fails.len();
    println!(
        "feedparser values: {correct}/{checked} correct ({supported} of {} files have a \
         machine-checkable Expect; {unmapped_files} use paths this harness does not map \
         — known gap)\n  top unmapped paths: {}",
        wf.len(),
        top_counts(&unmapped, 15)
    );
    assert!(
        FEEDPARSER_VALUE_CHECKED_FLOOR <= checked,
        "only {checked} value assertions were evaluated, was {FEEDPARSER_VALUE_CHECKED_FLOOR}. \
         Value checks were LOST — fix the mapping, do not lower the floor."
    );
    assert!(
        FEEDPARSER_VALUE_CORRECT_FLOOR <= correct,
        "{correct}/{checked} upstream value assertions hold, was \
         {FEEDPARSER_VALUE_CORRECT_FLOOR}. This is a REGRESSION.\n\
         Sample of the {} current failures:\n{}",
        fails.len(),
        fmt_fails(&fails)
    );
    if FEEDPARSER_VALUE_CORRECT_FLOOR < correct {
        println!(
            "NOTE: value conformance improved to {correct}/{checked}; raise \
             FEEDPARSER_VALUE_CORRECT_FLOOR (and both twins) to {correct}."
        );
    }

    // 3. The ill-formed half, with the enumerated exceptions asserted in
    //    both directions.
    let accepted_illformed = feedparser_accepted_illformed();
    let mut unexpectedly_accepted: Vec<String> = Vec::new();
    let mut no_longer_accepted: Vec<String> = Vec::new();
    for path in &ill_all {
        let key = rel_path(path)
            .strip_prefix(FEEDPARSER_ILL_PREFIX)
            .unwrap_or_default()
            .to_string();
        let listed = accepted_illformed.contains_key(key.as_str());
        let outcome = safe_parse(&parser, &read_file(path));
        if outcome.is_ok() && !listed {
            unexpectedly_accepted.push(format!("ACCEPTED but upstream marks it ill-formed: {key}"));
        }
        if outcome.is_err() && listed {
            no_longer_accepted.push(format!(
                "now REJECTED (good) — delete its entry from \
                 feedparser_accepted_illformed: {key}"
            ));
        }
    }
    println!(
        "feedparser illformed: {}/{} rejected ({} enumerated as outside a string-input \
         XML parser's reach)",
        ill_all.len() - accepted_illformed.len(),
        ill_all.len(),
        accepted_illformed.len()
    );
    assert!(
        unexpectedly_accepted.is_empty(),
        "must-reject failures:\n{}",
        fmt_fails(&unexpectedly_accepted)
    );
    assert!(
        no_longer_accepted.is_empty(),
        "feedparser_accepted_illformed is now stale:\n{}",
        fmt_fails(&no_longer_accepted)
    );

    // 4. Dialect and version detection. Both oracles come from the
    //    corpus, never from this crate: the document element read out of
    //    the source text, and the upstream `version == 'X'` annotation.
    let raw = conform_parser(FeedFormat::Raw);

    let mut fails: Vec<String> = Vec::new();
    for path in &wf {
        let src = read_file(path);
        let want = match root_local_name(&src).as_str() {
            "feed" => "atom",
            "rss" => "rss",
            "RDF" => "rdf",
            _ => "",
        };
        match safe_parse(&raw, &src) {
            Err(error) => fails.push(format!("{}: {error}", rel_path(path))),
            Ok(root) => {
                let got = detect(&root);
                if got.dialect.as_str() != want {
                    fails.push(format!(
                        "{}: {} (root <{}>)",
                        rel_path(path),
                        got.dialect.as_str(),
                        root_local_name(&src)
                    ));
                }
            }
        }
    }
    println!(
        "feedparser dialect: {}/{} correct",
        wf.len() - fails.len(),
        wf.len()
    );
    assert!(
        fails.is_empty(),
        "dialect failures ({}/{}):\n{}",
        fails.len(),
        wf.len(),
        fmt_fails(&fails)
    );

    let known_wrong = feedparser_version_known_wrong();
    let version_annotation = Regex::new(r"version == '([a-z0-9]+)'").expect("a valid pattern");
    let mut fails: Vec<String> = Vec::new();
    let mut stale: Vec<String> = Vec::new();
    let mut checked = 0usize;
    for path in &wf {
        let src = read_file(path);
        let Some(found) = version_annotation.captures(&src) else {
            continue;
        };
        checked += 1;
        let want = found[1].to_string();
        let key = rel_path(path)
            .strip_prefix(FEEDPARSER_WF_PREFIX)
            .unwrap_or_default()
            .to_string();
        let got = match safe_parse(&raw, &src) {
            Err(error) => format!("THREW: {error}"),
            Ok(root) => detect(&root).version.as_str().to_string(),
        };
        let listed = known_wrong.get(key.as_str()).copied();
        if got == want {
            if listed.is_some() {
                stale.push(format!(
                    "now correct — delete its feedparser_version_known_wrong entry: {key}"
                ));
            }
        } else if listed != Some(got.as_str()) {
            let extra = match listed {
                Some(known) => format!(" (recorded as {known})"),
                None => String::new(),
            };
            fails.push(format!("{key}: {got}, upstream says {want}{extra}"));
        }
    }
    assert!(0 < checked, "no version annotations found — corpus wrong?");
    println!(
        "feedparser version: {}/{} correct ({} enumerated disagreements)",
        checked - known_wrong.len(),
        checked,
        known_wrong.len()
    );
    assert!(
        fails.is_empty(),
        "version failures ({}/{}):\n{}",
        fails.len(),
        checked,
        fmt_fails(&fails)
    );
    assert!(
        stale.is_empty(),
        "feedparser_version_known_wrong is stale:\n{}",
        fmt_fails(&stale)
    );
}
