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
// modes (in `common::spec_runner`) and what an `ERROR:` cell means.
//
// EVERY row runs. There is no exemption list, because there is no row
// this port cannot satisfy: the two that could not, both of them pinning
// the rendered message of a namespace rejection raised by `tabnas-xml`,
// pass since that crate's Rust raise path was repaired. A row this port
// cannot satisfy belongs in `test/divergent.tsv`, executed by
// `tests/divergent_test.rs`, never in a skip here.

mod common;

use std::fs;

use tabnas_support::{load_spec_dir, SpecOptions};

use common::{spec_dir, spec_runner};

#[test]
fn every_shared_fixture() {
    let specs = load_spec_dir(spec_dir(), &SpecOptions::default()).expect("the fixtures load");
    assert!(
        !specs.is_empty(),
        "no fixtures under {}",
        spec_dir().display()
    );

    let mut failures: Vec<String> = Vec::new();
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
