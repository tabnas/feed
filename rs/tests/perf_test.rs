// A performance regression guard, mirroring go/perf_test.go and
// ts/test/perf.test.ts.
//
// Building a feed parser is expensive: the plugin pulls in the xml
// grammar and rebuilds the whole rule set, which dominates a parse. The
// documented usage is therefore to build one instance and reuse it, and
// `tabnas_feed::parse` keeps a shared one for exactly that reason.
//
// The comparison is machine-INDEPENDENT -- both sides scale together on a
// slow runner -- and there is deliberately no wall-clock budget.

mod common;

use std::time::Instant;

use common::default_parser;

#[test]
fn reusing_a_parser_beats_rebuilding_one() {
    const SRC: &str = "<rss version=\"2.0\"><channel><title>x</title>\
                       <item><title>i</title></item></channel></rss>";
    const N: usize = 200;

    // Warm both paths so the comparison is steady state.
    for _ in 0..10 {
        default_parser().parse(SRC).expect("warm rebuild parses");
    }
    let reused = default_parser();
    for _ in 0..10 {
        reused.parse(SRC).expect("warm reuse parses");
    }

    let start = Instant::now();
    for _ in 0..N {
        default_parser().parse(SRC).expect("rebuild parses");
    }
    let rebuild = start.elapsed();

    let start = Instant::now();
    for _ in 0..N {
        reused.parse(SRC).expect("reuse parses");
    }
    let reuse = start.elapsed();

    assert!(
        reuse * 4 <= rebuild,
        "reusing a feed parser is not meaningfully faster than rebuilding one per parse: \
         {N} reuse parses took {reuse:?} against {rebuild:?} rebuilding each time \
         (speedup {:.1}x, want at least 4x). Building the instance (the xml plugin and \
         the grammar) should dominate, so reuse should win by a wide margin.",
        rebuild.as_secs_f64() / reuse.as_secs_f64().max(f64::MIN_POSITIVE)
    );
}
