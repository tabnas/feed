# ci/

The Rust gate's script, kept here so that you can run the same gate
locally.

- `rust/run.sh` is the Rust gate. `.github/workflows/rust.yml` runs it,
  and so can you.

The workflows themselves live in `.github/workflows/`. To change CI, edit
them there in a reviewed pull request: session credentials can push
workflow changes (admin `DECISIONS.md` ADR-8, as amended on 2026-09-24),
so staging a workflow here for a maintainer to promote is optional.
Sessions still cannot push tags. Releases therefore go through
`workflow_dispatch`, and a workflow that runs only on a tag push needs a
maintainer to push that tag.

Five of this repository's workflows also have a template in admin
`rollout/workflows/`: `ci.yml`, `crates-release.yml`, `notify-status.yml`,
`release.yml` and `scorecard.yml`. ADR-8 as amended says a workflow
changed here is mirrored in its template, so change the template too, in
admin. Otherwise admin `scripts/verify.sh` reports the drift, and the
next `rollout/apply-workflows.sh --apply` pushes the old text back.
`clib.yml` and `clib-release.yml` are stamped (each carries a
`tabnas-clib-template` marker): they change only through admin
`tasks/clib-template/` and a re-stamp with `tasks/adopt-clib.sh`, never
by hand. The others have no template and change here alone.

## Promoted

Both of these were staged here and now run from `.github/workflows/`:

- **`docs.yml`** — the prose gate: Vale over the reader-facing pages at
  the levels set in `.vale.ini`, on the file list
  `ts/scripts/gated-docs.cjs` produces. See `docs/STYLE-GUIDE.md`.
  `make prose` runs the same check locally.

- **`rust.yml`** — the Rust port gate: format, build, tests,
  doctests and clippy with `-D warnings`, plus a lockfile comparison that
  exempts the sibling path crates. Everything it does lives in
  `ci/rust/run.sh`, so the workflow and a local run cannot say different
  things; `bash ci/rust/run.sh` is that local run.

  It needs the sibling checkouts (`parser`, `json`, `jsonic`, `xml`,
  `support`, `debug`) and no secrets. It clones them from `main` rather
  than from a release, which is what the Go and TypeScript jobs already
  do and what `test/spec/xml-layer.tsv` depends on.
