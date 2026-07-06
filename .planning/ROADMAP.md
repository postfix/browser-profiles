# Roadmap — Milestone v1.1: Hardening + Anti-detect Fidelity

> Numbering continues from v1.0 (phases 01–07). This roadmap uses phases 08–14.

## Overview

| # | Phase | Goal | Requirements | Success Criteria |
|---|---|---|---|---|
| 08 | Commit v1.0 Cutover | Land the uncommitted Go-port changes in git and make the working tree clean. | HARD-01 | 3 |
| 09 | Test Hardening | Raise coverage, add golden fixtures, Chrome-free tests, and detector-oracle baselines. | HARD-02, HARD-03, HARD-04, HARD-05, HARD-06 | 5 |
| 10 | Anti-Detect Modes | Wire the existing WebRTC / Canvas / Audio mode fields. | ADT-01, ADT-02, ADT-03 | 4 |
| 11 | Navigator Coherence | Make all navigator values coherent and add client hints + WebGL caps to the launch bundle. | ADT-04, ADT-05, ADT-06 | 5 |
| 12 | Permissions, Plugins, Fonts | Expand permissions, platform-normalise plugins, and add a lightweight fonts guard. | ADD-01, ADD-02, ADD-03 | 4 |
| 13 | WebGPU + Timing | Add WebGPU spoofing and gated timing/performance spoofing. | ADD-04, ADD-05 | 4 |
| 14 | Docs + Close | Update release notes, docs, and run the milestone close gates. | HARD-07 | 3 |

**Coverage:** 18 requirements → 7 phases → 100% mapped.

## Phase Details

### Phase 08: Commit v1.0 Cutover

**Goal:** Land the uncommitted v1.0 Go-port changes in git and make the working tree clean.

**Requirements:** HARD-01

**Success criteria:**
1. `git status --short` shows no pending changes except v1.1 planning artifacts (if any).
2. A git commit exists containing the full Go-port file set and the deletion of the `src/` TS tree.
3. `go build ./...`, `go vet ./...`, and `go test -count=1 ./...` pass on the committed tree.

**Depends on:** Nothing (first phase of the milestone).

---

### Phase 09: Test Hardening

**Status:** done

**Goal:** Raise coverage, add golden fixtures, Chrome-free tests, and detector-oracle baselines so later anti-detect changes are safe to make.

**Requirements:** HARD-02, HARD-03, HARD-04, HARD-05, HARD-06

**Success criteria:**
1. Root-package test coverage is ≥ 85% (measured by `go test -cover`). ✅ 85.1%
2. A golden-fixture checksum file guards every anti-detect script; any builder change fails CI until the fixture is updated. ✅
3. Chrome-free unit tests exist for timezone helpers, session-reuse probe, CLI happy paths, and the `PatchPage` subset. ✅
4. Integration tests run hermetically or skip gracefully when Chrome is unavailable; network calls for geo-IP and proxy are mocked. ✅
5. A detector-oracle baseline runs CreepJS, ThumbmarkJS, and BrowserLeaks checks against a launched profile and records scores before any anti-detect change is made. ✅

**Notes:** Root-package coverage reached 85.1%; remaining uncovered branches are real-Chrome launch paths. Detector-oracle baseline recorded in `.planning/data/09-detector-baseline.json`. See `.planning/phases/09-test-hardening/09-SUMMARY.md` for full results.

**Depends on:** Phase 08

---

### Phase 10: Anti-Detect Modes

**Goal:** Make the existing `FingerprintConfig` WebRTC / Canvas / Audio mode fields actually control behavior.

**Requirements:** ADT-01, ADT-02, ADT-03

**Success criteria:**
1. `FingerprintConfig.WebRTC` accepts `"disable"`, `"fake"`, and `"real"`; default matches v1.0 behavior.
2. `FingerprintConfig.Canvas` accepts `"noise"` and `"real"`; default matches v1.0 behavior.
3. `FingerprintConfig.Audio` accepts `"noise"` and `"real"`; default matches v1.0 behavior.
4. Golden fixtures exist for each mode variant and a combined `all/modes` fixture; checksum tests pass.

**Depends on:** Phase 09

---

### Phase 11: Navigator Coherence

**Goal:** Make every `navigator.*` value coherent with the profile persona and ensure client hints + WebGL caps are part of the launch bundle.

**Requirements:** ADT-04, ADT-05, ADT-06

**Success criteria:**
1. `navigator.appVersion`, `navigator.productSub`, `navigator.vendor`, `navigator.maxTouchPoints`, `navigator.mobile`, and `navigator.connection` are derived from the generated profile and consistent with `navigator.userAgent` and `navigator.platform`.
2. User-Agent Client Hints (`navigator.userAgentData` + `Sec-CH-UA*` headers) are injected into the core launch path, not just the consumer helper.
3. WebGL numeric capabilities (`MAX_TEXTURE_SIZE`, `MAX_VIEWPORT_DIMS`, etc.) are stable per profile and chosen from a GPU-family-specific table.
4. A real-Chrome smoke test verifies the three surface groups above; a third-party detector (CreepJS or BrowserLeaks) does not report contradictions.
5. Cross-tab / new-target injection preserves the same values (existing `installProtections` path verified).

**Depends on:** Phase 10

---

### Phase 12: Permissions, Plugins, Fonts

**Goal:** Add the lowest-risk differentiators: permissions API, platform-specific plugins, and a lightweight fonts guard.

**Requirements:** ADD-01, ADD-02, ADD-03

**Success criteria:**
1. `navigator.permissions.query` returns coherent states for `camera`, `microphone`, `geolocation`, and `notifications`.
2. `navigator.plugins` and `navigator.mimeTypes` expose a platform- and browser-specific list with correct array shapes and methods.
3. A `document.fonts` guard returns `true` for fonts in the configured per-OS whitelist and `false` otherwise; the limitation is documented.
4. Golden fixtures and real-Chrome smoke tests exist for each of the three surfaces.

**Depends on:** Phase 11

---

### Phase 13: WebGPU + Timing

**Goal:** Add forward-looking WebGPU spoofing and optional, gated timing/performance spoofing.

**Requirements:** ADD-04, ADD-05

**Success criteria:**
1. `navigator.gpu.requestAdapter` returns a mocked `GPUAdapter.info` consistent with the profile's GPU family.
2. WebGPU spoofing is golden-tested and verified in a real browser that supports WebGPU.
3. Timing spoofing is off by default; when enabled, `performance.now()` and `Date.now()` are rounded to the configured precision.
4. Optional CDP CPU throttling can be applied via profile settings without breaking default behavior.

**Depends on:** Phase 12

---

### Phase 14: Docs + Close

**Goal:** Update docs and release notes, then run the milestone close gates.

**Requirements:** HARD-07

**Success criteria:**
1. `README.md` and `docs/` reflect the new v1.1 surfaces, their caveats, and how to use mode flags.
2. `CHANGELOG.md` includes the v1.1 entry with detector-score improvements and deferred items.
3. Security review and milestone audit are completed with no blocking findings.

**Depends on:** Phase 13

---

## Records

```yaml
gate_result:
  record_author: architect
  accountable_owner: architect
  id: G-roadmap-v1.1
  kind: plan_gate
  gates_subject: roadmap v1.1 — requirements coverage and phase ordering
  evaluator: architect
  verdict: pass
  status: accepted
  method:
    - in-band roadmap authored from research findings (DETECTORS.md, SURFACES.md, INTEGRATION.md, PITFALLS.md, SUMMARY.md)
    - every requirement mapped to exactly one phase
    - 100% coverage of 18 requirements across 7 phases
```
