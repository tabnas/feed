// The divergence register: where this repo's ports DISAGREE, executed.
//
// WHY THIS IS NOT A FIXTURE. A fixture fails when behaviour REGRESSES.
// This fails BOTH ways: when a port is repaired to agree with another,
// the row still claims they differ, so the suite goes red and names the
// row to delete. A divergence recorded as a passing test of current
// behaviour survives its own repair, and the record then describes
// something that no longer happens, with nothing red.
//
// The file is `test/divergent.tsv`, not `test/spec/divergent.tsv`: all
// three parity runners enumerate `test/spec` and read the second column
// header to decide what a file asserts, so a register dropped in there
// would be refused by every one of them.
//
// The rows are run through the SAME runner the parity suite uses
// (`common::spec_runner`), so a cell here means what a cell there means,
// the `ERROR:<message fragment>` convention included.

mod common;

use tabnas_support::{load_spec, Register, SpecOptions};

use common::{repo_root, spec_runner};

#[test]
fn the_register_still_records_what_it_says() {
    let path = repo_root().join("test").join("divergent.tsv");
    let spec = load_spec(&path, &SpecOptions::default())
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));

    // An EMPTY register is legitimate, a repo with no divergences, but an
    // empty FILE is not: it cannot be told apart from a loader that read
    // nothing.
    assert!(!spec.rows.is_empty(), "{} has no rows", path.display());

    Register::new(spec_runner("expected"), "rust", &["ts", "go", "rust"]).spec(&spec);
}

/// Every row of the register must carry the `why` that sends a reader to
/// the repair, and `DIVERGENCE.md` must argue the same set.
#[test]
fn every_row_says_where_the_repair_lives() {
    let path = repo_root().join("test").join("divergent.tsv");
    let spec = load_spec(&path, &SpecOptions::default())
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    // The page wraps near 72 columns and a cell is one line, so a wrap
    // is not a difference in the text.
    let register = std::fs::read_to_string(repo_root().join("DIVERGENCE.md"))
        .expect("DIVERGENCE.md is readable")
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ");

    for row in &spec.rows {
        let why = row.named("why");
        assert!(
            !why.trim().is_empty(),
            "{}: the why column is empty",
            row.location()
        );
        let why_flat = why.split_whitespace().collect::<Vec<_>>().join(" ");
        assert!(
            register.contains(&why_flat),
            "{}: DIVERGENCE.md does not carry {why:?}",
            row.location()
        );
    }
}
