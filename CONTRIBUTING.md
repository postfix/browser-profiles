# Contributing

Thanks for your interest in `browser-profiles`.

## Development

Requires **Go 1.26+** and a local Chrome/Chromium (auto-detected, or set `CHROME_PATH`).

```bash
git clone https://github.com/postfix/browser-profiles
cd browser-profiles
go build ./...
go test ./...
```

## Project layout

- Root package `browserprofiles` — core types, filesystem profile store, the go-rod launcher
  (with the anti-automation flag handling), the authenticated forward proxy, and the session
  convenience constructors.
- `fingerprint/` — the anti-detect browser-side scripts (embedded byte-for-byte from the reference
  implementation, golden-string tested) and the fingerprint generator.
- `cmd/browser-profiles/` — the cobra CLI.

## Tests

`go test ./...`:

- **Golden tests** (`fingerprint/`) assert the injected protection scripts are byte-identical to
  the reference — do not edit `fingerprint/scripts/*.js` casually; the golden fixtures in
  `fingerprint/testdata/` are the parity contract.
- **Launcher / proxy** smoke tests exercise real headless Chrome and a local forward proxy; the
  Chrome-dependent tests skip automatically when no browser is available.

## Style

- Run `gofmt -w` before committing.
- In library code prefer go-rod's non-`Must` APIs (return errors); reserve `Must*` for the CLI.
