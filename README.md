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
| .epub           | META-INF/ directory | FLP-0013 | Yes               |
| Any other       | Sidecar .folio file | FLP-0015 | Adjacent file     |

The data model is identical across all formats.
The transport changes. The protocol does not.

---

## Quick Start

### Requirements

```
pandoc    — pandoc.org (single binary, free, no installer on Windows)
folio.exe — github.com/folioprotocol/folio-go/releases
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
folio/
├── README.md                        ← you are here
│
├── spec/                            ← open protocol specifications
│   ├── FLP-0000-protocol.md         ← the three primitives, format neutrality
│   ├── FLP-0001-data-model.md       ← JSON schema and field definitions
│   ├── FLP-0002-redline.md          ← diff operation vocabulary
│   ├── FLP-0003-markup.md           ← pen-holder model, sign-offs, disputes
│   ├── FLP-0004-integrity.md        ← Pandoc AST fingerprinting algorithm
│   └── FLP-0005-conformance.md      ← validation rules, known constraints
│
├── schema/
│   └── folio-record.schema.json     ← machine-readable JSON schema
│
├── transports/
│   └── TRANSPORTS.md                ← FLP-0010 DOCX, FLP-0011 ODF,
│                                       FLP-0012 PDF, FLP-0015 Sidecar
│
├── examples/
│   ├── valid/
│   │   ├── minimal.folio            ← smallest conforming record
│   │   └── full-lifecycle.folio     ← draft→markup→signoff→pdf→executed
│   └── invalid/
│       └── version-gap.folio        ← conformance test: version gap
│
├── docs/
│   ├── PRD.md                       ← product requirements document
│   ├── RATIONALE.md                 ← design decisions and tradeoffs
│   └── BUILD.md                     ← build and deployment instructions
│
├── folio-go/                        ← Go reference implementation
│   ├── go.mod                       ← zero external dependencies
│   ├── cmd/folio/main.go            ← CLI entry point
│   └── internal/
│       ├── core/record.go           ← data model (FLP-0001)
│       ├── fingerprint/fingerprint.go ← Pandoc AST pipeline (FLP-0004)
│       ├── transport/transport.go   ← DOCX, ODF, sidecar adapters
│       └── validate/validate.go     ← conformance checker (FLP-0005)
│
├── folio-dotnet/                    ← C# library (NuGet / Word add-in)
│   └── src/FolioDocument.cs         ← built on Open XML SDK
│
└── folio-js/                        ← TypeScript (Office.js add-in)
    └── src/folio-core.ts            ← Custom XML Parts API
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
