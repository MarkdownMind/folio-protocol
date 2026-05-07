# Folio Protocol

A universal document protocol. Any document. Any format. No server required.

```
urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08
```

---

## What It Does

Every document that enters the Folio ecosystem gets three things:

**Identity** — A permanent UUID that survives renaming, copying, format
conversion, and transfer between organizations. A DOCX and its converted
PDF share the same UUID. The document knows what it is.

**History** — An append-only, cryptographically fingerprinted version log
embedded inside the file itself. Send it anywhere — the history comes with it.

**Events** — A semantic log: initialized, versioned, sent, marked up,
signed off, converted, executed. Complete chain of custody, in the file,
no platform required to read it.

---

## Supported Formats

| Format          | Transport           | Spec     | Embedded in file? |
|-----------------|---------------------|----------|-------------------|
| .docx           | Custom XML Part     | FLP-0010 | Yes               |
| .odt .ods .odp  | META-INF/ directory | FLP-0011 | Yes               |
| .pdf            | XMP stream          | FLP-0012 | Yes               |
| .epub           | META-INF/ directory | FLP-0011 | Yes               |
| Any other       | Sidecar .folio file | FLP-0015 | Adjacent file     |

The data model is identical across all formats.
The transport changes. The protocol does not.

---

## Quick Start

### Install

```bash
# Download the folio binary for your platform from the latest release:
# https://github.com/MarkdownMind/folio-protocol/releases/latest

# macOS (Apple Silicon)
curl -Lo folio https://github.com/MarkdownMind/folio-protocol/releases/latest/download/folio-darwin-arm64
chmod +x folio && sudo mv folio /usr/local/bin/

# Linux (x86-64)
curl -Lo folio https://github.com/MarkdownMind/folio-protocol/releases/latest/download/folio-linux-amd64
chmod +x folio && sudo mv folio /usr/local/bin/

# Windows (x86-64) — in PowerShell
Invoke-WebRequest -Uri https://github.com/MarkdownMind/folio-protocol/releases/latest/download/folio-windows-amd64.exe -OutFile folio.exe
```

### Requirements

```
pandoc  — pandoc.org (single binary, free)
folio   — see install above
```

### Track any document

```bash
folio track contract.docx  --author ian@firm.com --title "NDA — Acme"
folio track brief.pdf      --author ian@firm.com
folio track notes.md       --author ian@firm.com
# Same command. Same record. Any format.
```

### Record versions

```bash
# After editing the document:
folio save contract.docx --note "Incorporated tax markup"

# See what changed:
folio history contract.docx

# Compare any two documents (even different formats):
folio redline contract_v1.docx contract_v2.odt

# Verify nothing changed since a version was recorded:
folio verify contract.docx
```

### Convert formats — chain of custody preserved

```bash
folio convert contract.docx contract_final.pdf --author ian@firm.com
# → UUID preserved across the format boundary
# → CONVERTED event recorded with fingerprint
# → Full history travels into the PDF
```

### Mark significant points

```bash
folio milestone contract_final.pdf --label "Executed"
```

---

## The Three Primitives

### Identity

```json
"identity": {
  "id":         "urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08",
  "title":      "Service Agreement — Acme Corp",
  "created":    "2026-05-03T10:00:00Z",
  "created-by": "ian@firm.com"
}
```

### History

```json
"history": [
  {
    "v":           1,
    "author":      "ian@firm.com",
    "timestamp":   "2026-05-03T10:00:00Z",
    "fingerprint": "sha256:e3b0c44298fc...",
    "ast-version": "3.2.1",
    "note":        "Initial draft",
    "format":      "docx"
  }
]
```

### Events

```json
"events": [
  { "event": "INITIALIZED",  "by": "ian@firm.com",  "timestamp": "..." },
  { "event": "SENT",         "to": "counsel@...",    "timestamp": "..." },
  { "event": "MARKUP_ADDED", "by": "counsel@...",    "timestamp": "..." },
  { "event": "INCORPORATED", "by": "ian@firm.com",   "timestamp": "..." },
  { "event": "SIGNED_OFF",   "by": "counsel@...",    "timestamp": "..." },
  { "event": "CONVERTED",    "from": "docx", "to": "pdf", "timestamp": "..." },
  { "event": "MILESTONE",    "label": "Executed",    "timestamp": "..." }
]
```

---

## Repository Structure

```
folio-protocol/
├── README.md
├── LICENSE                          ← MIT
│
│   ── Core specs (format-agnostic) ──────────────────────────────────
├── FLP-0000-protocol.md             ← the three primitives, format neutrality
├── FLP-0001-data-model.md           ← JSON schema and field definitions
├── FLP-0002-redline.md              ← diff operation vocabulary
├── FLP-0003-markup.md               ← pen-holder model, sign-offs, disputes
├── FLP-0004-integrity.md            ← Pandoc AST fingerprinting algorithm
├── FLP-0005-conformance.md          ← validation rules, known constraints
│
│   ── Transport specs ───────────────────────────────────────────────
├── FLP-0010-docx.md                 ← DOCX Custom XML Part
├── FLP-0011-odf.md                  ← ODF/EPUB META-INF/folio.json
├── FLP-0012-pdf.md                  ← PDF XMP stream
├── FLP-0015-sidecar.md              ← universal sidecar (.folio file)
│
│   ── Schema ────────────────────────────────────────────────────────
├── folio-record.schema.json         ← machine-readable JSON schema (FLP-0001)
│
│   ── Test corpus ───────────────────────────────────────────────────
├── minimal.folio                    ← smallest conforming record
├── full-lifecycle.folio             ← draft→markup→signoff→pdf→executed
├── version-gap.folio                ← conformance test: version gap (invalid)
├── testdata/
│   ├── valid-with-signoff.folio     ← markup incorporated, signed off
│   ├── valid-with-markup.folio      ← open markup (pending review)
│   ├── invalid-bad-uuid.folio       ← fails: URN format
│   ├── invalid-bad-fingerprint.folio← fails: fingerprint format
│   └── invalid-missing-author.folio ← fails: required field
│
│   ── Go reference implementation ──────────────────────────────────
├── go.mod
├── cmd/folio/main.go                ← CLI entry point (all commands)
├── internal/
│   ├── core/core.go                 ← data model (FLP-0001)
│   ├── fingerprint/fingerprint.go   ← Pandoc AST pipeline (FLP-0004)
│   ├── transport/transport.go       ← DOCX, ODF, PDF, sidecar adapters
│   └── validate/validate.go         ← conformance checker (FLP-0005)
│
│   ── Other reference implementations ──────────────────────────────
├── FolioDocument.cs                 ← C# / Open XML SDK
└── folio-core.ts                    ← TypeScript / Office.js
```

---

## Implementation Languages

| Component    | Language   | Why                                          |
|--------------|------------|----------------------------------------------|
| folio-go     | Go         | Single binary, zero runtime, cross-compiles  |
|              |            | to .exe/.bin with `GOOS= GOARCH= go build`   |
| folio-dotnet | C#         | NuGet ecosystem, Open XML SDK, Word add-in   |
|              |            | backend, AOT single-file .exe                |
| folio-js     | TypeScript | Office.js add-in (task pane), browser        |
| Spec/schema  | Language-  | JSON schema, Markdown — implement in         |
|              | agnostic   | any language                                 |

Python is deliberately absent. Law firm Windows machines will not have
Python installed. Go and C# compile to single binaries with no runtime
dependency. That is the deployment reality.

---

## How Fingerprinting Works

```
any document
    ↓ pandoc (system binary)
Pandoc AST JSON (format-agnostic semantic representation)
    ↓ strip metadata noise (dates, revision counters, generator fields)
    ↓ normalize whitespace in text nodes
    ↓ sort JSON keys (deterministic serialization)
    ↓ SHA-256
sha256:{64 hex chars}
```

The same content in DOCX and ODT produces the same fingerprint.
Format conversions are cryptographically verifiable.

---

## Specification Index

### Core (format-agnostic)
| Spec     | Title                         |
|----------|-------------------------------|
| FLP-0000 | Protocol Statement            |
| FLP-0001 | Data Model & Event Schema     |
| FLP-0002 | Redline Operation Vocabulary  |
| FLP-0003 | Markup and Sign-off Model     |
| FLP-0004 | Cryptographic Integrity Model |
| FLP-0005 | Conformance and Validation    |

### Transport (format-specific)
| Spec     | Title                         |
|----------|-------------------------------|
| FLP-0010 | DOCX Transport                |
| FLP-0011 | ODT/ODF Transport             |
| FLP-0012 | PDF Transport (XMP)           |
| FLP-0013 | EPUB Transport (planned)      |
| FLP-0014 | HTML/Markdown (planned)       |
| FLP-0015 | Sidecar Transport             |

### Extensions (planned)
| Spec     | Title                         |
|----------|-------------------------------|
| FLP-0016 | Cryptographic Signing         |
| FLP-0017 | AI Narration Interface        |
| FLP-0018 | Multi-document Relationships  |

---

## Prior Art

**Pandoc** — format conversion engine. Folio uses Pandoc's AST as its
content model and calls pandoc as a subprocess. Folio adds the protocol
layer (identity, history, collaboration) that Pandoc does not provide.

**Git** — version control protocol. Folio borrows git's concepts
(identity, append-only history, semantic diff, merge with conflict
detection) while solving git's document-specific limitations: binary
format opacity, no embedded history, developer-only UX.

**Simul Docs / Version Story** — cloud platforms for document version
control. Folio differs by being a protocol (not a platform), local-first
(not cloud-dependent), and format-agnostic (not DOCX-only).

---

## License

MIT. Spec, schema, and all reference implementations.

---

## Status

**Alpha — spec draft, implementations in progress.**

Not production ready. Contributions welcome:
- Implementations in additional languages (Rust, Java)
- Transport adapter implementations (PDF via pdfcpu)
- Real-world testing with complex documents
- Spec review and ambiguity reports
- Test corpus documents
