# test/specs

Cross-language test fixtures for `@tabnas/feed`. Each spec is a pair (or
triple) of files sharing a base name:

```
<name>.xml             # input feed XML
<name>.atom.json       # expected output for default ('atom') format
<name>.native.json     # optional: expected output for 'native' format
<name>.detect.json     # optional: expected { dialect, version }
```

Both the TypeScript and Go test suites enumerate this directory, parse
each `.xml` with the corresponding format option, and compare the
result to the expected JSON via structural deep-equal (after a
JSON marshal/unmarshal round-trip to normalise types and ordering).

Add a new spec by dropping in the three files; both languages will
pick it up automatically. Keep the inputs minimal — exercises one
piece of behaviour each — so failures pinpoint a single mapping.
