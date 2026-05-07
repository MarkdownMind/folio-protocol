# Folio Protocol — Product Requirements Document
**Version:** 2.0  
**Status:** Draft  
**Date:** May 2026  
**Replaces:** PRD v1.0 (DOCX-only scope)

---

## 1. Executive Summary

Folio is a universal document protocol. It defines how any document —
regardless of format, application, or platform — carries its own 
identity, history, and collaboration state as an intrinsic property 
of the file itself.

Three primitives. No server required. No platform required.

**Identity** — A permanent UUID that persists across versions, renames,
copies, and format conversions.

**History** — An append-only, cryptographically fingerprinted version 
log embedded in the document itself.

**Events** — A semantic log of everything that happened to the document 
from initialization to final execution.

The protocol is format-agnostic. Transport adapters define how it 
embeds in DOCX, ODT, PDF, EPUB, Markdown, and any other format. 
The data model is identical regardless of host format.

---

## 2. The Problem

Every person who works with documents has five unsolved pains:

1. **No identity** — A document has no persistent identity beyond its 
   filename. Rename it, copy it, convert it — it becomes unrecognizable.

2. **No portable history** — Version history lives on servers. Send the 
   file to a counterparty and the history stays behind.

3. **No format continuity** — A contract drafted as DOCX and executed as 
   PDF are two unconnected objects. Nobody knows they're the same document.

4. **No approval integrity** — There's no way to prove what a document 
   said when someone signed off on it.

5. **No universal diff** — You cannot meaningfully compare a DOCX to an 
   ODT. You cannot compare any document to its PDF. The formats are 
   opaque to each other.

These pains exist across every profession, every industry, every 
document format. Folio solves all five with three primitives.

---

## 3. Why Now

**AI made format opacity painful.** When people started feeding documents 
to AI, they discovered PDF and DOCX are black boxes. AI has to guess 
structure from noise. Folio's semantic fingerprinting layer is also the 
machine-readable content layer AI needs.

**Pandoc matured.** An open-source, MIT-licensed, 40-format converter 
with a stable JSON AST exists and works. Folio doesn't need to build 
the conversion layer — it builds the protocol layer on top of it.

**The format war is over.** DOCX, ODF, and PDF all coexist permanently. 
The industry doesn't need a new format. It needs a protocol that works 
across all of them. That's a different and achievable problem.

---

## 4. Architecture

### 4.1 Layer Model

```
┌──────────────────────────────────────────────────────────────┐
│                    USER APPLICATIONS                          │
│  Word · LibreOffice · Acrobat · VS Code · Any editor         │
├──────────────────────────────────────────────────────────────┤
│                  FOLIO TOOLING LAYER                          │
│  folio-cli · Word add-in · LibreOffice ext · API server      │
├──────────────────────────────────────────────────────────────┤
│                  FOLIO LIBRARY LAYER                          │
│  folio-core (Python) · folio-dotnet (C#) · folio-js (JS)     │
├──────────────────────────────────────────────────────────────┤
│               TRANSPORT ADAPTERS                              │
│  DOCX(FLP-0010) ODT(FLP-0011) PDF(FLP-0012) Sidecar(FLP-0015)│
├──────────────────────────────────────────────────────────────┤
│               CONTENT MODEL                                   │
│  Pandoc AST JSON — format-agnostic semantic representation   │
├──────────────────────────────────────────────────────────────┤
│               OPEN PROTOCOL SPEC                              │
│  FLP-0000 through FLP-0005 · MIT · github.com/folioprotocol  │
└──────────────────────────────────────────────────────────────┘
```

### 4.2 Content Model: Pandoc AST

Folio uses the Pandoc Abstract Syntax Tree as its canonical content 
representation for diffing and fingerprinting.

- Open source, MIT licensed, actively maintained (jgm/pandoc)
- Reads and writes 40+ document formats
- JSON output is language-agnostic
- Battle-tested across millions of real documents

**Known limitation:** Pandoc's AST is less expressive than DOCX/ODT 
for formatting details (margins, fonts, complex tables). Folio accepts 
this tradeoff because it fingerprints and diffs **content meaning**, 
not visual appearance. The original format file is always preserved 
intact alongside the Folio record.

### 4.3 Transport Adapters

| Format     | Spec     | Mechanism                      | Host App          |
|------------|----------|--------------------------------|-------------------|
| DOCX       | FLP-0010 | Custom XML Part (ISO 29500)    | Word              |
| ODT/ODF    | FLP-0011 | META-INF/ directory (ISO 26300)| LibreOffice       |
| PDF        | FLP-0012 | XMP custom namespace (ISO 16684)| Acrobat, any viewer|
| EPUB       | FLP-0013 | META-INF/ directory (OCF 3.2)  | Any EPUB reader   |
| MD/TXT/etc | FLP-0014 | Sidecar .folio file            | Any editor        |
| Any format | FLP-0015 | Sidecar .folio file (fallback) | Any app           |

All transports carry the same JSON record structure. The data model 
is defined once in FLP-0001 and does not change per format.

---

## 5. The Folio Record

A Folio record is a JSON document. Always UTF-8. Always the same 
structure. Three objects:

```json
{
  "folio": "1.0",
  "identity": {
    "id":         "urn:folio:{uuid-v4}",
    "title":      "Contract title",
    "created":    "ISO8601 UTC",
    "created-by": "author identifier"
  },
  "history": [
    {
      "v":           1,
      "author":      "ian@firm.com",
      "timestamp":   "ISO8601 UTC",
      "fingerprint": "sha256:{64 hex chars}",
      "ast-version": "3.2.1",
      "note":        "human readable",
      "format":      "docx"
    }
  ],
  "collaboration": {
    "pen-holder": "ian@firm.com",
    "markups":    [],
    "signoffs":   []
  }
}
```

---

## 6. Fingerprinting

```
fingerprint = "sha256:" + hex(SHA256(canonicalize(pandoc_ast_json(document))))
```

For PDF: hash the content streams directly (not Pandoc AST).

The fingerprint is format-agnostic: the same content in DOCX and ODT 
produces the same fingerprint. This makes format conversions 
cryptographically verifiable — you can prove no content was lost.

---

## 7. Key Workflows

### 7.1 Universal Tracking (any format)

```bash
folio track contract.docx --author ian@firm.com
folio track brief.pdf     --author ian@firm.com
folio track notes.md      --author ian@firm.com
# Same command. Same record structure. Different transports.
```

### 7.2 Format-Preserving Conversion

```bash
folio convert contract.docx contract.odt --author ian@firm.com
# → Converts using Pandoc
# → Preserves document UUID
# → Records CONVERTED event
# → New fingerprint computed for ODT
# → Complete chain of custody across the format boundary
```

### 7.3 Cross-Format Verification

```bash
folio verify contract.odt
# ✓ Fingerprint verified — matches version 2
#   Content matches the DOCX fingerprint recorded at conversion.
```

### 7.4 Redline Across Formats

```bash
folio redline contract_v1.docx contract_v2.odt
# Works regardless of format difference
# Both converted to Pandoc AST, diffed semantically
# Output: human-readable change summary
```

### 7.5 The Full Legal Lifecycle

```
Day 1:  folio track contract.docx       → INITIALIZED, v1
Day 3:  folio save contract.docx        → VERSIONED, v2 (after edits)
Day 5:  [specialist returns edits]      → MARKUP_ADDED
Day 7:  folio save contract.docx        → INCORPORATED, VERSIONED, v3
Day 10: [tax counsel signs off]         → SIGNED_OFF, fingerprint anchored
Day 14: folio convert contract.docx \
              contract_final.pdf        → CONVERTED, same UUID, v4
Day 14: folio milestone contract_final.pdf --label "Executed"
```

One UUID. One record. Complete chain.

---

## 8. Implementation Plan

### Phase 1 — Open Protocol Foundation (Months 1–3)
**Goal:** The spec and reference implementation exist and work.

- [ ] Publish FLP-0000 through FLP-0005 on GitHub (MIT)
- [ ] Publish transport specs FLP-0010, FLP-0011, FLP-0012, FLP-0015
- [ ] Publish JSON schema (folio-record.schema.json)
- [ ] Ship folio-core (Python) on PyPI
- [ ] Ship folio-cli as standalone tool
- [ ] Publish test corpus (valid + invalid examples)
- [ ] Hacker News, document format communities, Pandoc community

**Success metric:** 10+ GitHub stars, Pandoc community engagement, 
1 independent implementation attempt.

### Phase 2 — DOCX Tooling (Months 3–6)
**Goal:** Works in Word for legal users.

- [ ] folio-dotnet: C# library on NuGet (Open XML SDK)
- [ ] Folio for Word: Office.js add-in, AppSource submission
- [ ] Word Online degradation handling (read-only mode)
- [ ] Beta: 1–2 law firms using it for real documents

**Success metric:** AppSource listed, 1 law firm as beta user.

### Phase 3 — ODT and LibreOffice (Months 5–8)
**Goal:** Works in LibreOffice, opens Folio to non-Microsoft users.

- [ ] folio-odf: Python/Java ODF transport adapter
- [ ] LibreOffice extension (Basic or Python macro)
- [ ] Cross-format redline: DOCX ↔ ODT comparison working
- [ ] ODT community outreach (TDF/LibreOffice community)

**Success metric:** LibreOffice extension published, ODT transport spec 
adopted by at least 1 LibreOffice contributor.

### Phase 4 — AI Narration and Local Node (Months 6–12)
**Goal:** Plain-English diff narration, privacy-first hardware play.

- [ ] folio-narrate: LLM prompt layer over redline ops
- [ ] Local node: Mac Mini M4, Gemma 3 12B, fine-tuned on document ops
- [ ] Hardware bundle: pre-configured, plug-in-and-go
- [ ] AI narration in Word add-in sidebar
- [ ] First paying customer

**Success metric:** First hardware sale ($2,500 + $200/month).

### Phase 5 — Platform Applications (Month 12+)
**Goal:** Vertical applications built on Folio as infrastructure.

- [ ] PetitionIQ rebuilt on Folio protocol layer
- [ ] Engineering/construction vertical (submittal tracking)
- [ ] API server (folio-server) for enterprise integration
- [ ] iManage / NetDocuments connector

---

## 9. Competitive Position

| Product        | Protocol? | Format-agnostic? | Local-first? | Open spec? |
|----------------|-----------|-----------------|--------------|------------|
| Version Story  | No        | No (DOCX only)  | No           | No         |
| Simul Docs     | No        | No (DOCX only)  | No           | No         |
| SharePoint     | No        | No              | No           | No         |
| iManage        | No        | Partial         | No           | No         |
| Git            | Yes       | Yes (text)      | Yes          | Yes        |
| **Folio**      | **Yes**   | **Yes**         | **Yes**      | **Yes**    |

Git is the closest analogue — it's a protocol, not a platform, and 
it's format-agnostic. Folio is git for documents: same architectural 
philosophy, different vocabulary, binary format support, no terminal.

---

## 10. Technical Constraints

| Constraint | Detail | Source |
|------------|--------|--------|
| Word Online CustomXmlParts write instability | Degrade to read-only | github.com/OfficeDev/office-js/issues/5910 |
| Word Online 1MB part size limit | Compact at 750KB | github.com/OfficeDev/office-js/issues/2408 |
| Pandoc AST lossiness for formatting | Acceptable — content only | pandoc.org/CONTRIBUTING.html |
| Pandoc version sensitivity | Record ast-version per version record | FLP-0004 §5 |
| ODF META-INF/ files need no manifest entry | Clean injection point | ISO 26300-3 |
| XMP custom namespaces fully supported | PDF transport clean | ISO 16684-1:2019 |
| EPUB META-INF/ allows unknown files | Same adapter as ODF | OCF 3.2 spec |

---

## 11. Open Questions

1. **Pandoc version pinning** — Should the protocol require a specific 
   minimum Pandoc version? Or record and accept any version?

2. **Sidecar vs embedded precedence** — When both exist and conflict, 
   embedded wins (current spec). Is this always correct?

3. **Multi-document relationships** — A master agreement with exhibits. 
   How does Folio represent document families? (FLP-0018 placeholder)

4. **Offline-first sync** — Two people edit locally, reconnect. How 
   does Folio handle merge conflicts on the record itself (not just 
   the document content)? Similar to git's merge model.

5. **Record size growth** — For long-lived documents with hundreds of 
   versions, the embedded record grows. Compaction strategy needs 
   formal spec (archive older records to separate part/file).

---

## 12. Success Metrics

| Metric                        | Phase 1 | Phase 2 | Phase 4 |
|-------------------------------|---------|---------|---------|
| GitHub stars (spec repo)      | 50      | 200     | 1,000   |
| Supported formats             | 4       | 6       | 10+     |
| External implementations      | 0       | 1–2     | 5+      |
| Beta organizations            | 0       | 1–2     | 10+     |
| Paying customers (hardware)   | 0       | 0       | 5+      |
| AppSource installs            | 0       | 50      | 500+    |

---

## 13. References

**Standards:**
- ISO/IEC 29500: OOXML (DOCX host format)
- ISO/IEC 26300: ODF (ODT host format)  
- ISO 16684-1:2019: XMP (PDF metadata)
- EPUB OCF 3.2: EPUB container format
- RFC 4122: UUID v4
- FIPS 180-4: SHA-256
- W3C Canonical XML 2.0: https://www.w3.org/TR/xml-c14n2/

**Libraries:**
- Pandoc: pandoc.org / github.com/jgm/pandoc (MIT)
- Open XML SDK: github.com/dotnet/Open-XML-SDK (MIT)
- pikepdf: github.com/pikepdf/pikepdf (MIT)
- odfpy: github.com/eea/odfpy (Apache 2.0)

**Research:**
- TDF DOCX complexity analysis: blog.documentfoundation.org
- FSFE OOXML critique: fsfe.org/activities/msooxml
- Version Story research: theredline.versionstory.com
- Pandoc AST lossiness: pandoc.org/CONTRIBUTING.html
- Word Online CustomXmlParts bugs: github.com/OfficeDev/office-js
