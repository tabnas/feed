// The engine's error carries a code, position, hint and a formatted
// report, so `Result<_, TabnasError>` trips clippy's `result_large_err`;
// an integration test is a crate of its own, so it allows it again.
#![allow(clippy::result_large_err)]

// A parse from a reader, a window of the source at a time (admin ADR-30;
// tabnas/parser's doc/chunked-source.md, "Proving it"), against the same
// text parsed whole. `make()` declares `lex.window`: the tokens are
// tabnas-xml's matcher's, which reads the source only through the lexer's
// window, and this plugin's one hook converts the finished element tree,
// so a reader is read a window at a time.
//
// Every row of the shared fixtures and every document of the three feed
// corpora (feedparser-wellformed, feedvalidator and feedparser) is parsed
// whole and from a reader that hands its bytes over in pieces of 1, 3 and
// 64 bytes, under the engine's window and under windows whose margin and
// block are a few characters, so that every construct meets the window's
// edge; a long feed, whose long tokens run past the engine's window, in
// pieces of 4 KiB too. The value, the tokens the lexer gives, the rule
// passes, and an error's code, position, row, column, token and length
// must be the same. An error's excerpt is not compared: a parse from a
// reader shows a long line only as far back as the window reaches
// (tabnas/parser's DIVERGENCE.md).
//
// ts/test/window.test.ts and go/window_test.go hold the TypeScript and Go
// ports to the same.

mod common;

use std::io::Read;
use std::sync::{Arc, Mutex};

use tabnas::{RuleState, Tabnas, TabnasError, Value, WindowSizes};
use tabnas_feed::{FeedFormat, FeedOptions};
use tabnas_support::{load_spec_dir, SpecOptions};

use common::{read_file, repo_root, require_corpus, spec_dir, walk_xml, wellformed_dir};

/// A reader that hands its bytes over at most `size` at a time.
struct Pieces<'a> {
    bytes: &'a [u8],
    at: usize,
    size: usize,
}

impl Read for Pieces<'_> {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        let n = self.size.min(buf.len()).min(self.bytes.len() - self.at);
        buf[..n].copy_from_slice(&self.bytes[self.at..self.at + n]);
        self.at += n;
        Ok(n)
    }
}

/// A read of a source: the pieces a reader hands its bytes over in, and the
/// window it is read through, the engine's own (64 KiB) where it is `None`,
/// or one whose margin and block are a few characters, which the hidden
/// `parse_reader_with_sizes` sets, its reads as small as the pieces.
type Reading = (usize, Option<(usize, usize)>);

const PIECES: [usize; 3] = [1, 3, 64];
const WINDOWS: [Option<(usize, usize)>; 4] = [None, Some((1, 1)), Some((3, 2)), Some((64, 64))];

/// A fixture row is read in every pieces size through every window.
fn row_reads() -> Vec<Reading> {
    PIECES
        .iter()
        .flat_map(|&size| WINDOWS.iter().map(move |&window| (size, window)))
        .collect()
}

/// A corpus document in pieces of 1, 3 and 64 bytes through the engine's
/// window, and through windows of 1 and 1, 3 and 2, and 64 and 64
/// characters in pieces of 1, 3 and 64.
const DOCUMENT_READS: [Reading; 6] = [
    (1, None),
    (3, None),
    (64, None),
    (1, Some((1, 1))),
    (3, Some((3, 2))),
    (64, Some((64, 64))),
];

/// A long feed in pieces of 1, 3, 64 and 4096 bytes through the engine's
/// window, whose edges its long tokens straddle, and through a window of 64
/// and 64 characters.
const LONG_READS: [Reading; 5] = [
    (1, None),
    (3, None),
    (64, None),
    (4096, None),
    (64, Some((64, 64))),
];

type Log = Arc<Mutex<(Vec<String>, Vec<String>)>>;

/// `parser` with a log of the tokens the lexer gives and the rule passes.
fn logged(mut parser: Tabnas, log: &Log) -> Tabnas {
    let tokens = Arc::clone(log);
    parser.subscribe_tokens(move |token| {
        tokens.lock().unwrap().0.push(format!(
            "{} {:?} {} {} {}:{}",
            token.tin, token.src, token.val, token.site.pos, token.site.ri, token.site.ci
        ));
    });
    let rules = Arc::clone(log);
    parser.subscribe_rule_done(move |rule, _context, done| {
        let state = match done.state {
            RuleState::Open => "open",
            RuleState::Close => "close",
        };
        rules
            .lock()
            .unwrap()
            .1
            .push(format!("{} {state} {}", rule.name.as_str(), rule.d));
    });
    parser
}

/// What a parse showed: its value, or its error without the excerpt; and
/// the tokens and rule passes `log` gathered, which is empty again after.
fn seen(log: &Log, parsed: Result<Value, TabnasError>) -> (String, Vec<String>, Vec<String>) {
    let outcome = match parsed {
        Ok(value) => format!("ok {value}"),
        Err(e) => format!(
            "err {} at {} {}:{} {:?} {} {:?} {:?} {:?} {:?} {:?}",
            e.code,
            e.pos,
            e.row,
            e.col,
            e.src,
            e.len,
            e.token,
            e.rule,
            e.rule_stack,
            e.expected,
            e.detail
        ),
    };
    let (tokens, rules) = std::mem::take(&mut *log.lock().unwrap());
    (outcome, tokens, rules)
}

/// `text` parsed whole and from a reader in each of `reads`; the count of
/// reader parses compared.
fn differential(parser: &Tabnas, log: &Log, at: &str, text: &str, reads: &[Reading]) -> usize {
    let whole = seen(log, parser.parse(text));
    for &(size, window) in reads {
        let reader = Pieces {
            bytes: text.as_bytes(),
            at: 0,
            size,
        };
        let parsed = match window {
            None => parser.parse_reader(reader),
            Some((margin, block)) => parser.parse_reader_with_sizes(
                reader,
                WindowSizes {
                    margin,
                    block,
                    chunk: size,
                },
            ),
        };
        let read = seen(log, parsed);
        assert_eq!(read, whole, "{at} in pieces of {size}, window {window:?}");
    }
    reads.len()
}

/// A feed of every kind of token, the long ones longer than the engine's
/// 64 KiB window, and characters past ASCII among them, so that tokens of
/// every kind straddle the window's edges as a reader hands the text over.
/// The same document as `longFeed` in ts/test/window.test.ts and
/// go/window_test.go.
fn long_feed() -> String {
    let mut parts: Vec<String> = Vec::new();
    parts.push("\u{FEFF}<?xml version=\"1.0\" encoding=\"utf-8\"?>\n".into());
    parts.push("<!DOCTYPE feed [\n".into());
    for i in 0..1300 {
        parts.push(format!(
            "  <!ENTITY e{i} \"entity \u{20ac} {i}\"> <!-- {i} > ] ' -->\n"
        ));
    }
    parts.push("]>\n".into());
    parts.push(format!(
        "<!-- {}-->\n",
        "a comment past the window, \u{e9} \u{1d11e} - ".repeat(2000)
    ));
    parts.push("<feed xmlns=\"http://www.w3.org/2005/Atom\" xml:lang=\"en\">\n".into());
    parts.push("  <title type=\"text\">A long feed &amp; its entries</title>\n".into());
    parts.push("  <id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>\n".into());
    parts.push("  <updated>2026-11-09T12:00:00Z</updated>\n".into());
    parts.push(format!(
        "  <link rel=\"self\" href=\"http://example.org/feed?{}\"/>\n",
        "q=a%20long%20query%20past%20the%20window&amp;".repeat(1600)
    ));
    for i in 0..1000 {
        parts.push(format!(
            "  <entry>\n    <title>Entry {i} &#{}; t\u{eb}xt</title>\n    <id>urn:uuid:{i}</id>\n    \
             <updated>2026-11-09T12:00:00Z</updated>\n    <author><name>Au\u{f0}or {i}</name></author>\r\n    \
             <summary type=\"html\">&lt;p&gt;summary {i}&lt;/p&gt;</summary>\n  </entry>\n",
            60 + (i % 40)
        ));
        if i == 500 {
            parts.push(format!(
                "  <entry>\n    <title>The long one</title>\n    <id>urn:uuid:long</id>\n    \
                 <updated>2026-11-09T12:00:00Z</updated>\n    \
                 <content type=\"html\"><![CDATA[{}]]></content>\n    \
                 <summary>{}</summary>\n    <?pi {}?>\n    <category term=\"t\" label=\"{}\"/>\n  </entry>\n",
                "<b>cdata past the window</b> \u{e7} ]] > ".repeat(2200),
                "character data past the window, \u{e5} &amp; &lt;x&gt; \r\n".repeat(1300),
                "an instruction past the window \u{f1} ? > ".repeat(1800),
                "a label past the window \u{fc} ".repeat(2600)
            ));
        }
    }
    parts.push("</feed>\n<!-- after -->\n".into());
    parts.concat()
}

#[test]
fn make_reads_a_reader_a_window_at_a_time() {
    assert!(tabnas_feed::make().options.lex.window);
    let native = FeedOptions {
        format: FeedFormat::Native,
        ..FeedOptions::default()
    };
    assert!(tabnas_feed::make_with(&native).options.lex.window);
    // The plugin itself declares nothing.
    let mut parser = Tabnas::new();
    parser
        .use_plugin(tabnas_feed::plugin(), None)
        .expect("the plugin installs");
    assert!(!parser.options.lex.window);
}

#[test]
fn every_fixture_row_parses_the_same_from_a_reader() {
    let log: Log = Arc::default();
    let files = load_spec_dir(spec_dir(), &SpecOptions::default()).expect("the fixtures load");
    let mut rows = 0;
    let mut compared = 0;
    for (index, file) in files.iter().enumerate() {
        let mode = file.header.get(1).map(String::as_str).unwrap_or_default();
        for row in &file.rows {
            let options = if mode == "detect" {
                FeedOptions {
                    format: FeedFormat::Raw,
                    ..FeedOptions::default()
                }
            } else {
                let opts = row.named("opts");
                if opts.trim().is_empty() {
                    FeedOptions::default()
                } else {
                    let json: serde_json::Value =
                        serde_json::from_str(opts).expect("the opts column is JSON");
                    FeedOptions::from_value(&Value::from_json(&json))
                }
            };
            let parser = logged(tabnas_feed::make_with(&options), &log);
            let input = row.unesc_named("input");
            compared += differential(&parser, &log, &row.location(), &input, &row_reads());
            rows += 1;
        }
        eprintln!(
            "window fixtures: {} of {} files ({}%), {compared} reader parses",
            index + 1,
            files.len(),
            (index + 1) * 100 / files.len()
        );
    }
    assert!(rows > 50, "{rows}");
    assert_eq!(compared, rows * row_reads().len());
}

#[test]
fn every_corpus_document_parses_the_same_from_a_reader() {
    let log: Log = Arc::default();
    let parser = logged(tabnas_feed::make(), &log);
    let wellformed = wellformed_dir();
    assert!(wellformed.is_dir(), "{}", wellformed.display());
    let mut files = walk_xml(&wellformed);
    files.extend(walk_xml(&require_corpus(
        "feedvalidator",
        "fetch-feedvalidator.sh",
    )));
    files.extend(walk_xml(&require_corpus(
        "feedparser",
        "fetch-feedparser.sh",
    )));
    assert!(files.len() > 3500, "{}", files.len());
    let root = repo_root();
    let mut compared = 0;
    for (index, file) in files.iter().enumerate() {
        let text = read_file(file);
        let at = file
            .strip_prefix(&root)
            .unwrap_or(file)
            .display()
            .to_string();
        compared += differential(&parser, &log, &at, &text, &DOCUMENT_READS);
        if index % 1000 == 0 || index + 1 == files.len() {
            eprintln!(
                "window corpora: {} of {} documents ({}%), {compared} reader parses",
                index + 1,
                files.len(),
                (index + 1) * 100 / files.len()
            );
        }
    }
}

#[test]
fn a_long_feed_parses_the_same_from_a_reader() {
    let log: Log = Arc::default();
    let parser = logged(tabnas_feed::make(), &log);
    let text = long_feed();
    assert!(text.len() > 512 * 1024, "{}", text.len());
    let value = parser.parse(&text).expect("the long feed parses");
    log.lock().unwrap().0.clear();
    log.lock().unwrap().1.clear();
    assert_eq!(
        value.to_json()["entries"].as_array().map(Vec::len),
        Some(1001)
    );
    assert_eq!(
        differential(&parser, &log, "the long feed", &text, &LONG_READS),
        LONG_READS.len()
    );
}

#[test]
fn a_long_document_fails_the_same_from_a_reader() {
    // A construct that runs past the engine's window and is then refused,
    // and a long document that is no feed: the error is the whole text's.
    let log: Log = Arc::default();
    let parser = logged(tabnas_feed::make(), &log);
    let pad = format!("<title>{}</title>", "x\u{e9} ".repeat(30000));
    let bad = [
        format!(
            "<feed xmlns=\"http://www.w3.org/2005/Atom\">{pad}<!-- {}",
            "c ".repeat(40000)
        ),
        format!(
            "<feed xmlns=\"http://www.w3.org/2005/Atom\">{pad}<subtitle>{}]]></subtitle></feed>",
            "t ".repeat(40000)
        ),
        format!("<kml>{pad}</kml>"),
        format!(
            "<rss version=\"2.0\"><channel>{pad}</channel></rss>{}",
            "trailing ".repeat(10000)
        ),
    ];
    let mut compared = 0;
    for (index, text) in bad.iter().enumerate() {
        let whole = seen(&log, parser.parse(text));
        assert!(
            whole.0.starts_with("err "),
            "{}",
            &whole.0[..200.min(whole.0.len())]
        );
        compared += differential(
            &parser,
            &log,
            &format!("long error document {index}"),
            text,
            &LONG_READS,
        );
    }
    assert_eq!(compared, bad.len() * LONG_READS.len());
}
