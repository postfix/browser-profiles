# Vendored: ThumbmarkJS (test-only)

`thumbmark.umd.js` is the ThumbmarkJS browser-fingerprinting library, vendored **for
tests only** — it is NOT compiled into or shipped by this module.

It is used as an **independent fingerprinting oracle**: an integration test launches a
protected profile with go-rod + real Chrome, runs ThumbmarkJS inside the page, and asserts
that our anti-detect injection is observable by a third-party fingerprinter (distinct
per-profile identities; spoofed navigator/webgl values read back).

- Source: https://cdn.jsdelivr.net/npm/@thumbmarkjs/thumbmarkjs/dist/thumbmark.umd.js
- Upstream: https://github.com/thumbmarkjs/thumbmarkjs
- License: MIT (see `LICENSE`, © 2024 Ilkka)
- Retrieved: 2026-07-06
- SHA-256: `5ada488a4a77730944f5e76b0d4590b4caedf74893ebc362eb6e606f301bc0d8`
- UMD global: `window.ThumbmarkJS`; primary API `new ThumbmarkJS.Thumbmark(opts).get()` → `{ thumbmark, components }`.

To refresh: re-download from the CDN URL, update the SHA-256 above, and re-run the tests.
