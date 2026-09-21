# ci/

Staging area for GitHub Actions workflow changes.

This directory exists because session credentials cannot write
`.github/workflows/*` — see admin `DECISIONS.md` ADR-8. To change CI:

1. Put the intended workflow file in `workflows/`.
2. A maintainer promotes it with the admin `rollout/apply-ci-folders.sh`
   script.

## Pending

- **`workflows/docs.yml`** — the prose gate: Vale over the reader-facing
  pages at the levels set in `.vale.ini`, on the file list
  `ts/scripts/gated-docs.cjs` produces. See `docs/STYLE-GUIDE.md`.

  It needs no sibling checkouts and no secrets, and pins its own Vale
  version. Errors fail the job; warnings go to the run summary as a
  report. `make prose` runs the identical check locally, and the test
  suite already runs the other half of the gate
  (`ts/test/docs.test.js`), so promoting this adds the spelling and
  Google-convention arm rather than the whole gate.

- **`workflows/rust.yml`** — the Rust port gate: format, build, tests,
  doctests and clippy with `-D warnings`, plus a lockfile comparison that
  exempts the sibling path crates. Everything it does lives in
  `ci/rust/run.sh`, so this file and a local run cannot say different
  things; `bash ci/rust/run.sh` is that local run.

  It needs the sibling checkouts (`parser`, `json`, `jsonic`, `xml`,
  `support`, `debug`) and no secrets. It clones them from `main` rather
  than from a release, which is what the Go and TypeScript jobs already
  do and what `test/spec/xml-layer.tsv` depends on.
