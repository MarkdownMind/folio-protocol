# FLP-0005: Conformance and Validation
**Version:** 1.0.0-draft  
**Status:** Draft  
**Depends on:** FLP-0000 through FLP-0004

---

## 1. Conformance Levels

### Level 1: Reader
A tool that can read and display Folio data. Cannot write.

MUST:
- Parse `folio-record.schema.json` and validate against it
- Display version history
- Display pending markups
- Display stale sign-off warnings (FLP-0003 §6)
- Verify fingerprints on request (FLP-0004)

MUST NOT modify any Folio data.

### Level 2: Writer (Full Conformance)
A tool that can read and write Folio data.

All Level 1 requirements, plus MUST:
- Initialize Folio records on new documents
- Write version records on user save
- Compute fingerprints per FLP-0004
- Detect disputes per FLP-0003 §7
- Enforce markup status lifecycle per FLP-0003 §3
- Validate records before writing (Section 3)

---

## 2. Known Platform Constraints

### 2.1 Word Online — CustomXmlParts Write Instability

**Affects:** folio-js (Office.js) running in Word Online (browser)  
**Status:** Known platform issue as of 2026  
**Source:** github.com/OfficeDev/office-js/issues/5910

`customXmlParts.addAsync` and `.delete` fail silently in some
Word Online versions. Word Desktop (Windows and macOS) is reliable.

**Required behavior for Level 2 Word add-ins:**

```typescript
if (isWordOnline()) {
  // Degrade to Level 1 — read-only
  // Display: "Version tracking requires Word Desktop.
  //           History is visible but cannot be updated here."
}
```

### 2.2 CustomXmlParts Size Limit

**Affects:** DOCX transport (FLP-0010) on Word Online  
**Constraint:** 1MB hard limit  
**Source:** github.com/OfficeDev/office-js/issues/2408

The Folio record embedded in a DOCX MUST NOT exceed 900KB.
Implementations MUST monitor part size and warn at 750KB.

**Compaction strategy:** When approaching the limit, archive version
records older than 90 days (or oldest beyond the 50 most recent) to
a second part or sidecar file. The main record retains the manifest
and recent history. Compacted records remain auditable.

### 2.3 Pandoc Version Sensitivity

Fingerprints computed with different Pandoc versions may differ even
for identical content if the AST format changed between versions.

Implementations MUST:
- Record `ast-version` in every version record
- Warn when verifying if current Pandoc version differs from stored
- Never silently fail fingerprint verification due to version mismatch

### 2.4 Pandoc Requirement

All Level 2 implementations require Pandoc installed as a system binary.
Pandoc is available at pandoc.org as a single binary with no runtime
dependencies on any platform.

The folio-go CLI checks for pandoc availability and surfaces a clear
error message if not found:

```
✗ pandoc not found: install from pandoc.org
  pandoc is required for document fingerprinting
```

---

## 3. Validator Requirements

The reference validator (`folio validate <document>`) MUST check:

**Identity**
- [ ] `id` matches `urn:folio:{uuid-v4}` pattern
- [ ] `created` is valid ISO8601 UTC
- [ ] `created-by` is non-empty

**History**
- [ ] At least one version record exists
- [ ] Version numbers are sequential starting at 1 (no gaps)
- [ ] All `fingerprint` values match `sha256:[64 hex chars]`
- [ ] All `timestamp` values are valid ISO8601 UTC
- [ ] All `author` values are non-empty
- [ ] All `ast-version` values are non-empty

**Collaboration**
- [ ] No duplicate markup IDs
- [ ] No duplicate signoff IDs
- [ ] All markup `base-version` values reference existing versions
- [ ] All signoff `version` values reference existing versions
- [ ] All markup `base-fingerprint` values are valid format
- [ ] All signoff `fingerprint-at-signoff` values are valid format
- [ ] All markup `status` values are valid enum members

**Transport-specific (DOCX)**
- [ ] Part size is below 900KB

---

## 4. Test Corpus

All conforming implementations MUST pass the tests in:
`github.com/folioprotocol/spec/examples/`

```
examples/
  valid/
    minimal.folio           — manifest + 1 version, no collaboration
    full-lifecycle.folio    — complete lifecycle: markup, signoff,
                              conversion, milestone
  invalid/
    version-gap.folio       — v1 then v3, missing v2
```

Additional corpus documents to be added in v1.1:
```
  valid/
    cross-format.folio      — CONVERTED event chain docx→odt→pdf
    stale-signoff.folio     — sign-off with changed content
    with-disputes.folio     — conflicting markups
  invalid/
    bad-fingerprint.folio   — malformed fingerprint format
    bad-uuid.folio          — malformed document ID
    future-schema.folio     — schema version 99.0
```

---

## 5. Implementation Registry

A community registry of conforming implementations is maintained at:
`github.com/folioprotocol/implementations`

Implementers self-certify by running the test corpus and publishing
results. There is no formal certification process for v1.0.

---

## 6. Reference Implementations

| Implementation | Language   | Transport          | Conformance |
|----------------|------------|--------------------|-------------|
| folio-go       | Go         | DOCX, ODF, Sidecar | Level 2     |
| folio-dotnet   | C#         | DOCX, ODF, Sidecar | Level 2     |
| folio-js       | TypeScript | DOCX (Office.js)   | Level 2 (Level 1 on Word Online) |

All reference implementations are MIT licensed.
Source: github.com/folioprotocol/folio-go  
        github.com/folioprotocol/folio-dotnet  
        github.com/folioprotocol/folio-js
