# Test fixtures

`seeds/` is a small, frozen stand-in for the repo's `seeds/` directory, in the same file formats.
Service, DAL, adapter and composition tests and the e2e specs load it instead of the real campaign data, so editing
`seeds/` never breaks them. Change a fixture value only when a test needs it.

The real `seeds/` is checked only by `internal/seedcheck` (loads, schema-valid, references resolve).

`catalogue.json` is the app's stand-in for the private spell/rules catalogue: the book's numbers and ids,
invented placeholder text (never book wording). `TestFixture_NumbersMatchTheBuiltCatalogue`
(`internal/cataloguecheck`) compares every number it shares with the real catalogue; it is skipped, with a
message, when `.catalogue/catalogue.json` is absent. `just check-fixture-numbers` fetches the catalogue and
fails on any difference, naming the entry and field.
