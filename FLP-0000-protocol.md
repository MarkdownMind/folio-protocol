# FLP-0000: The Folio Protocol
**Version:** 1.0.0-draft  
**Status:** Draft  
**License:** MIT  
**Namespace:** `http://schemas.folioprotocol.io/v1`  
**Repository:** github.com/folioprotocol/spec

---

## 1. What Folio Is

Folio is a universal document protocol.

It defines how any document — regardless of format, application, or 
platform — carries its own identity, history, and collaboration state 
as an intrinsic property of the file itself.

A Folio document knows:
- **What it is** — a permanent identity that survives renaming, copying, 
  and format conversion
- **What it was** — a complete, append-only history of every version
- **Who touched it** — authorship and timestamps for every change
- **What happened to it** — a semantic event log from creation to execution

No server required. No platform required. No account required. Just the 
document.

---

## 2. The Three Primitives

Every Folio implementation, in every format, in every language, reduces 
to three things:

### 2.1 Identity

A permanent UUID assigned once, never changed:

```
urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08
```

This UUID persists across version changes, format conversions, renames, 
copies, and transfers. It is the document's passport. It makes the 
otherwise-impossible possible: proving that a PDF is the executed version 
of a DOCX that was derived from an earlier DOCX.

### 2.2 State

At any moment, a Folio document has a queryable state:

```json
{
  "id":          "urn:folio:a3f9c821-...",
  "version":     3,
  "fingerprint": "sha256:e3b0c44298fc...",
  "author":      "ian@firm.com",
  "timestamp":   "2026-05-03T14:32:00Z",
  "status":      "in-review",
  "format":      "docx"
}
```

Any conforming tool — a CLI, a Word add-in, a document management system, 
a court filing system — reads the same state from the same embedded record.

### 2.3 Events

A Folio document is an append-only log of semantic events:

```
INITIALIZED  → who started tracking, from what document, when
VERSIONED    → content fingerprint, author, note
SENT         → to whom, which version, when
MARKUP_ADDED → proposed changes, from whom, against which version
INCORPORATED → which markups, into which new version
SIGNED_OFF   → by whom, on which version, content hash at that moment
CONVERTED    → from format, to format, same UUID, new fingerprint
MILESTONE    → label, version, who marked it
```

The event log is the complete chain of custody. It cannot be backdated. 
It cannot be falsified without breaking the cryptographic fingerprint 
chain. It is the document's life story.

---

## 3. Format Neutrality

Folio is not a DOCX protocol. It is not a PDF protocol. It is not a 
Microsoft protocol or an Adobe protocol.

The Folio data model is defined independently of any document format. 
Transport adapters define how the protocol embeds in each format:

```
┌──────────────────────────────────────────────────────────┐
│              FOLIO PROTOCOL (format-agnostic)             │
│   FLP-0000 · FLP-0001 · FLP-0002 · FLP-0003 · FLP-0004  │
├──────┬───────┬──────┬─────────┬────────┬─────────────────┤
│ DOCX │  ODT  │ PDF  │  EPUB   │   MD   │    Sidecar      │
│      │  ODF  │      │         │  TXT   │   (.folio)      │
│FLP   │ FLP   │ FLP  │  FLP    │  HTML  │    FLP-0015     │
│0010  │ 0011  │ 0012 │  0013   │ FLP-   │                 │
│      │       │      │         │  0014  │                 │
└──────┴───────┴──────┴─────────┴────────┴─────────────────┘
```

The core specs (FLP-0000 through FLP-0005) define behavior. The transport 
specs (FLP-0010 onwards) define embedding. An implementation targeting only 
DOCX implements FLP-0000 through FLP-0005 plus FLP-0010. An implementation 
targeting all formats implements all specs.

---

## 4. The Pandoc AST as Content Model

Folio uses the Pandoc Abstract Syntax Tree (pandoc-types) as its canonical 
content representation for diffing and fingerprinting.

**Why Pandoc AST:**
- Open source, MIT licensed, actively maintained
- Reads and writes 40+ document formats
- Format-agnostic semantic representation
- JSON output mode enables language-agnostic processing
- Widely implemented (Haskell, Python, Lua, R, JavaScript bindings)
- Battle-tested across millions of real documents

**The pipeline:**

```
any document format
    ↓ pandoc reader
Pandoc AST (JSON)
    ↓ strip format noise (rsids, revision markers, etc.)
Canonical AST
    ↓ SHA-256
fingerprint
```

```
any document format
    ↓ pandoc reader  
Pandoc AST (JSON) for version N
Pandoc AST (JSON) for version M
    ↓ semantic diff (Myers on AST nodes)
semantic op log (FLP-0002)
    ↓ optional AI narration
plain English summary
```

**Known limitation:** Pandoc's intermediate representation is less 
expressive than many of the formats it converts between. It preserves 
structural elements but not formatting details such as margin size. Some 
document elements, such as complex tables, may not fit into Pandoc's 
simple document model. (pandoc.org/CONTRIBUTING.html)

This is acceptable for Folio's purposes because:
1. Folio diffs and fingerprints **content**, not formatting
2. The original format file is always preserved intact
3. Formatting is preserved in the format-specific file, not in the Folio record
4. For legal documents, content changes are what matter; formatting changes 
   are surfaced separately as `format` operations in FLP-0002

---

## 5. The Sidecar Model

For formats that cannot embed metadata (plain text, CSV, proprietary binary 
formats, or any format where the user doesn't want to modify the original):

```
contract.docx           ← original, untouched
contract.docx.folio     ← Folio record, adjacent file
```

The `.folio` sidecar is a valid Folio record in standalone JSON format. 
It is the fallback transport for any format and the primary transport for 
formats that cannot embed metadata.

When both an embedded record and a sidecar exist, the embedded record 
takes precedence. Tools SHOULD warn if they conflict.

---

## 6. The Chain-of-Custody Guarantee

The killer feature that no other protocol provides:

A document can start as DOCX, be negotiated as ODT by a counterparty, 
be executed as PDF, be archived as a scanned image — and the entire 
chain is one continuous Folio record with one permanent UUID.

```
contract_v1.docx   [INITIALIZED, VERSIONED v1]
    ↓ email to counterparty
contract_v1.docx   [SENT to opposing@counsel.com]
    ↓ counterparty edits in LibreOffice
contract_v1.odt    [MARKUP_ADDED from opposing@counsel.com]
    ↓ pen-holder incorporates
contract_v2.docx   [INCORPORATED, VERSIONED v2]
    ↓ tax counsel reviews
contract_v2.docx   [SIGNED_OFF by tax@counsel.com, fingerprint anchored]
    ↓ convert to PDF for execution
contract_final.pdf [CONVERTED from docx, same UUID, VERSIONED v3]
    ↓ parties sign
contract_final.pdf [MILESTONE "Executed", all parties recorded]
```

One UUID. One event log. Complete chain. Cryptographically verifiable at 
any point. No platform required to read it — the record is in the file.

---

## 7. Relationship to Existing Standards

| Standard         | Relationship to Folio                              |
|------------------|----------------------------------------------------|
| ISO/IEC 29500    | OOXML — host format for DOCX transport (FLP-0010) |
| ISO/IEC 26300    | ODF — host format for ODT transport (FLP-0011)    |
| ISO 16684        | XMP — host format for PDF transport (FLP-0012)    |
| EPUB OCF 3.2     | Container — EPUB transport uses FLP-0011 model    |
| Pandoc AST       | Content model for diffing and fingerprinting       |
| W3C C14N 2.0     | Canonicalization before fingerprinting             |
| RFC 4122         | UUID v4 for document identity                      |
| FIPS 180-4       | SHA-256 for fingerprinting                         |

Folio does not compete with any of these. It adds a protocol layer that 
works with all of them.

---

## 8. Non-Goals

Folio is explicitly NOT:

- **A document format** — it embeds in existing formats
- **A cloud platform** — it works with no network connection
- **A DRM system** — it does not restrict access
- **A signing standard** — it records events, not legal signatures 
  (optional FLP-0016 will address signing)
- **A replacement for Word or LibreOffice** — users keep their tools
- **A replacement for SharePoint or iManage** — it works alongside them

---

## 9. Spec Index

### Core Specs (format-agnostic)
| Spec     | Title                            | Status |
|----------|----------------------------------|--------|
| FLP-0000 | Protocol Statement (this doc)    | Draft  |
| FLP-0001 | Data Model & Event Schema        | Draft  |
| FLP-0002 | Redline Operation Vocabulary     | Draft  |
| FLP-0003 | Markup and Sign-off Model        | Draft  |
| FLP-0004 | Cryptographic Integrity Model    | Draft  |
| FLP-0005 | Conformance and Validation       | Draft  |

### Transport Specs (format-specific)
| Spec     | Title                            | Status  |
|----------|----------------------------------|---------|
| FLP-0010 | DOCX Transport (customXml/)      | Draft   |
| FLP-0011 | ODT/ODF Transport (META-INF/)    | Draft   |
| FLP-0012 | PDF Transport (XMP stream)       | Draft   |
| FLP-0013 | EPUB Transport (META-INF/)       | Planned |
| FLP-0014 | HTML/Markdown Transport          | Planned |
| FLP-0015 | Sidecar Transport (.folio)       | Draft   |

### Extension Specs (optional features)
| Spec     | Title                            | Status  |
|----------|----------------------------------|---------|
| FLP-0016 | Cryptographic Signing            | Planned |
| FLP-0017 | AI Narration Interface           | Planned |
| FLP-0018 | Multi-document Relationships     | Planned |
