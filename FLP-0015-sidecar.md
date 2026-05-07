# FLP-0015: Sidecar Transport

**Status:** Draft  
**Depends on:** FLP-0000, FLP-0001  
**Formats:** Any file type (format-agnostic)  

---

## Overview

The sidecar transport stores the Folio record in a separate `.folio` file
adjacent to the tracked document. It is the universal fallback for any
format without a native embedding transport.

A sidecar is always valid. It is never preferred over an embedded
transport when one is available, but it is never wrong.

---

## Sidecar File Naming

The sidecar file name is the document name with `.folio` appended:

```
contract.docx     → contract.docx.folio
report.pdf        → report.pdf.folio
notes.md          → notes.md.folio
budget.xlsx       → budget.xlsx.folio
any_file.any_ext  → any_file.any_ext.folio
```

The sidecar is always in the **same directory** as the tracked document.

---

## Format

A sidecar file is a UTF-8 encoded JSON object conforming to the Folio
record schema (FLP-0001). It is identical in structure to an embedded
record — there is no sidecar-specific wrapper.

```json
{
  "identity": {
    "id": "urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08",
    "title": "Budget Q4 2026",
    "created": "2026-05-03T10:00:00Z",
    "created-by": "ian@firm.com"
  },
  "history": [ ... ],
  "events":  [ ... ]
}
```

---

## When Sidecar is Used

| Scenario                          | Transport       |
|----------------------------------|-----------------|
| DOCX file                        | FLP-0010 (DOCX) |
| ODT/ODS/ODP file                 | FLP-0011 (ODF)  |
| EPUB file                        | FLP-0011 (ODF)  |
| PDF file (current)               | FLP-0015        |
| XLSX, PPTX, MD, TXT, any other   | FLP-0015        |
| DOCX but Google Docs workflow    | FLP-0015        |

The `folio` CLI selects transport automatically by file extension. Users
do not configure this; it is transparent.

---

## Standalone `.folio` Files

A `.folio` file with no adjacent document is a standalone record. This
is used for test fixtures and record exchange without the document body.

```
corpus/
└── minimal.folio       ← standalone record (no .docx/.pdf alongside it)
```

Standalone `.folio` files are valid. They are also used by the `folio-go`
test corpus. The `FolioTransport` in `internal/transport/transport.go`
reads standalone `.folio` files directly.

---

## Separation Risk

The sidecar file can become separated from its document (renamed copy
without sidecar, attachment without sidecar, etc.). Implementations MUST
warn when a sidecar is present without a matching document, or a document
appears to have had a sidecar that is now missing.

Heuristic: if `contract.docx` has a INITIALIZED event but no embedded
record and no sidecar, display a warning that the tracking record may be lost.

---

## Backup and Sync

Sidecar files must be included in backup, sync, and version control.
Add `.folio` files to git, Dropbox, OneDrive, etc. alongside documents.

`.gitignore` must not exclude `*.folio` files.

---

## Access via Go (folio-go)

```go
// Read
record, err := transport.For("document.xlsx").Read("document.xlsx")
// → reads document.xlsx.folio

// Write
err = transport.For("document.xlsx").Write("document.xlsx", record)
// → writes document.xlsx.folio
```

For standalone `.folio` reading:
```go
record, err := transport.For("minimal.folio").Read("minimal.folio")
// → reads minimal.folio directly (FolioTransport, not SidecarTransport)
```

See `internal/transport/transport.go` for the full implementation.
