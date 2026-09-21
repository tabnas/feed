// The shared conformance fixtures, every one of them.
//
// `test/spec/*.tsv` is the parity contract: the TypeScript suite
// (`ts/test/parity.test.ts`), the Go suite (`go/parity_test.go`) and this
// file run the same rows through the shared runner. Files are discovered
// by listing, exactly as the other two runners discover them, so adding a
// fixture runs it in all three runtimes without touching any runner. A
// row green in one runtime and red in another is a failure, not a
// discrepancy.
//
// What is left here is only what is specific to feed: the two fixture
// modes (in `common::spec_runner`), what an `ERROR:` cell means, and the
// two rows this port cannot satisfy, which are recorded in
// `test/divergent.tsv` and executed by `tests/divergent_test.rs` instead.

mod common;

use std::fs;

use tabnas_support::{load_spec_dir, SpecOptions};

use common::{spec_dir, spec_runner};

/// Rows this port does NOT satisfy, by fixture and by input.
///
/// Each one is recorded in `test/divergent.tsv` with a `rust` column and
/// argued in `DIVERGENCE.md`; the register fails when the divergence is
/// repaired as loudly as when it regresses, so an entry here cannot
/// outlive the reason for it. The list is asserted to be EXACTLY what the
/// run met, so a stale entry fails too.
///
/// Both are the same defect in `tabnas-xml`'s Rust crate: the namespace
/// failure path never interpolates its message template, so the rendered
/// message is a placeholder while the code is correct. These fixtures pin
/// the MESSAGE, so they cannot pass until that is repaired upstream.
/// Keyed by the `opts` cell as well as the input, because the same input
/// appears twice in `xml-layer.tsv`: once with `strictNamespaces: false`,
/// where it parses and the row passes, and once with the defaults, where
/// it is refused and the message is what diverges.
const RECORDED_DIVERGENCES: [(&str, &str, &str); 2] = [
    (
        "xml-layer.tsv",
        "<feed xmlns=\"http://www.w3.org/2005/Atom\"><dc:language>en</dc:language></feed>",
        "",
    ),
    (
        "xml-layer.tsv",
        "<feed xmlns=\"http://www.w3.org/2005/Atom\" xmlns:a=\"http://x\ny\"/>",
        "",
    ),
];

#[test]
fn every_shared_fixture() {
    let specs = load_spec_dir(spec_dir(), &SpecOptions::default()).expect("the fixtures load");
    assert!(
        !specs.is_empty(),
        "no fixtures under {}",
        spec_dir().display()
    );

    let mut failures: Vec<String> = Vec::new();
    let mut met: Vec<(String, String, String)> = Vec::new();
    let mut rows = 0usize;

    for spec in &specs {
        let mode = spec.header.get(1).map(String::as_str).unwrap_or_default();
        assert!(
            mode == "expected" || mode == "detect",
            "{}: unknown second column {mode:?}",
            spec.file
        );
        let runner = spec_runner(mode);
        // The same guard `Runner::spec` applies: a fixture that loads but
        // holds no rows is a silent pass.
        runner
            .check_spec(spec)
            .unwrap_or_else(|error| panic!("{error}"));

        let probe = &spec.rows[0];
        let input_col = probe
            .resolve(runner.input_column())
            .unwrap_or_else(|error| panic!("{}: {error}", spec.file));
        let expected_col = probe
            .resolve(&tabnas_support::Column::Name(mode.to_string()))
            .unwrap_or_else(|error| panic!("{}: {error}", spec.file));

        for row in &spec.rows {
            rows += 1;
            let input = row.unesc(input_col);
            let opts = row.named("opts").to_string();
            if RECORDED_DIVERGENCES
                .iter()
                .any(|(file, text, cell)| *file == spec.file && *text == input && *cell == opts)
            {
                met.push((spec.file.clone(), input, opts));
                continue;
            }
            if let Err(error) = runner.check_row(row, &input, row.col(expected_col)) {
                failures.push(error.0);
            }
        }
    }

    assert!(
        failures.is_empty(),
        "{} fixture row(s) failed:\n{}",
        failures.len(),
        failures.join("\n")
    );
    assert!(0 < rows, "the fixtures hold no rows");

    // A recorded divergence whose row is gone is a stale exemption, and
    // an exemption list that is longer than what the run met would hide a
    // row that silently stopped running.
    let mut seen: Vec<(&str, &str, &str)> = met
        .iter()
        .map(|(file, input, opts)| (file.as_str(), input.as_str(), opts.as_str()))
        .collect();
    seen.sort_unstable();
    let mut declared: Vec<(&str, &str, &str)> = RECORDED_DIVERGENCES.to_vec();
    declared.sort_unstable();
    assert_eq!(
        seen, declared,
        "the recorded divergences and the rows met do not match; \
         delete a stale entry or add the row back"
    );
}

/// The runner reads the row by column NAME, so every fixture must carry
/// the named columns it reads; a file that does not would be run against
/// the wrong cells rather than refused. This is the tripwire the other
/// two runners get from their loaders.
#[test]
fn every_fixture_has_the_named_columns() {
    let mut names: Vec<String> = fs::read_dir(spec_dir())
        .expect("the spec directory lists")
        .filter_map(|entry| entry.ok())
        .map(|entry| entry.file_name().to_string_lossy().into_owned())
        .filter(|name| name.ends_with(".tsv"))
        .collect();
    names.sort();
    assert!(
        !names.is_empty(),
        "no fixtures under {}",
        spec_dir().display()
    );
    for name in &names {
        let path = spec_dir().join(name);
        let body = fs::read_to_string(&path).unwrap_or_else(|error| panic!("{name}: {error}"));
        let header: Vec<&str> = body
            .lines()
            .next()
            .unwrap_or_default()
            .split('\t')
            .collect();
        assert!(
            header.first() == Some(&"input"),
            "{name}: header {header:?} does not start with an input column"
        );
        assert!(
            header.get(1) == Some(&"expected") || header.get(1) == Some(&"detect"),
            "{name}: header {header:?} has no expected or detect column"
        );
    }
}
