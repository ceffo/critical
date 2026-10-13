# Test fixtures

`seeds/` is a small, frozen stand-in for the repo's `seeds/` directory, in the same file formats.
Service, DAL, adapter and composition tests and the e2e specs load it instead of the real campaign data, so editing
`seeds/` never breaks them. Change a fixture value only when a test needs it.

The real `seeds/` is checked only by `internal/seedcheck` (loads, schema-valid, references resolve).
