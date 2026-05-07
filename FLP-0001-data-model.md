# FLP-0001: Data Model and Event Schema
**Version:** 1.0.0-draft  
**Status:** Draft  
**Depends on:** FLP-0000

---

## 1. The Folio Record

A Folio Record is a JSON document that travels with or alongside a 
document file. It contains four top-level objects:

```json
{
  "folio": "1.0",
  "identity": { ... },
  "history":  [ ... ],
  "collaboration": { ... }
}
```

The record is always valid JSON. It is always UTF-8 encoded. It is 
always the same structure regardless of what format the document is in.

---

## 2. Identity Object

```json
{
  "id":         "urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08",
  "title":      "Service Agreement — Acme Corp",
  "created":    "2026-05-03T10:00:00Z",
  "created-by": "ian@firm.com",
  "lineage":    []
}
```

### 2.1 Fields

| Field        | Type     | Required | Description                               |
|--------------|----------|----------|-------------------------------------------|
| `id`         | string   | MUST     | `urn:folio:` + UUID v4, permanent         |
| `title`      | string   | SHOULD   | Human-readable document title             |
| `created`    | ISO8601  | MUST     | UTC timestamp of first initialization     |
| `created-by` | string   | MUST     | Email or identifier of original author    |
| `lineage`    | array    | MAY      | Parent document IDs if derived from others|

### 2.2 ID Permanence

The `id` MUST NOT change under any circumstances including:
- Document renamed
- Document copied
- Document converted to another format
- Document content changed
- Version incremented

The `id` is the document's identity across its entire lifetime.

### 2.3 Lineage

When a document is derived from another (a template instantiated, a 
contract based on a prior contract), the parent document's ID MAY be 
recorded in `lineage`. This creates an auditable chain of provenance.

---

## 3. History Array

The history is an append-only array of version records. Records MUST 
appear in ascending version order. Records MUST NOT be modified or 
removed after creation.

```json
"history": [
  {
    "v":           1,
    "author":      "ian@firm.com",
    "timestamp":   "2026-05-03T10:00:00Z",
    "fingerprint": "sha256:e3b0c44298fc1c149afb...",
    "ast-version": "3.2.1",
    "note":        "Initial draft",
    "format":      "docx"
  },
  {
    "v":           2,
    "author":      "ian@firm.com",
    "timestamp":   "2026-05-03T14:32:00Z",
    "fingerprint": "sha256:a87ff679a2f3e71d9181...",
    "ast-version": "3.2.1",
    "note":        "Incorporated tax counsel markup",
    "format":      "docx"
  }
]
```

### 3.1 Version Record Fields

| Field         | Type     | Required | Description                              |
|---------------|----------|----------|------------------------------------------|
| `v`           | integer  | MUST     | Monotonically increasing from 1, no gaps |
| `author`      | string   | MUST     | Who created this version                 |
| `timestamp`   | ISO8601  | MUST     | UTC timestamp                            |
| `fingerprint` | string   | MUST     | `sha256:` + 64 hex chars (see FLP-0004)  |
| `ast-version` | string   | MUST     | Pandoc API version used for fingerprint  |
| `note`        | string   | SHOULD   | Human-readable description of changes    |
| `format`      | string   | SHOULD   | File format at time of this version      |

### 3.2 Version Monotonicity

Version numbers MUST start at 1 and increment by exactly 1. Gaps are 
a conformance violation. Implementations MUST reject records with gaps.

### 3.3 ast-version

The Pandoc API version used when computing the fingerprint. This is 
critical because the Pandoc AST JSON format changes between major 
versions. Recording the version ensures fingerprints are reproducible.

Retrieve with: `pandoc --version | head -1`  
Or via API: the `pandoc-api-version` field in JSON AST output.

---

## 4. Events Array (within History)

In addition to version records, the history MAY contain event records 
that are not version increments:

```json
{
  "event":     "SENT",
  "timestamp": "2026-05-03T11:00:00Z",
  "by":        "ian@firm.com",
  "to":        "taxcounsel@partnerlaw.com",
  "version":   1,
  "channel":   "email"
}
```

```json
{
  "event":     "CONVERTED",
  "timestamp": "2026-05-04T09:00:00Z",
  "by":        "ian@firm.com",
  "from":      "docx",
  "to":        "pdf",
  "version":   3,
  "fingerprint-after": "sha256:c4ca4238a0b923..."
}
```

```json
{
  "event":     "MILESTONE",
  "timestamp": "2026-05-04T09:20:00Z",
  "by":        "ian@firm.com",
  "label":     "Executed",
  "version":   3
}
```

### 4.1 Event Types

| Event         | Description                                        |
|---------------|----------------------------------------------------|
| `INITIALIZED` | First time Folio tracking started on this document |
| `VERSIONED`   | New content version saved (also a history record)  |
| `SENT`        | Document sent to a party for review                |
| `MARKUP_ADDED`| Proposed changes submitted (see FLP-0003)          |
| `INCORPORATED`| Markups accepted into new version                  |
| `SIGNED_OFF`  | Party approved a specific version                  |
| `CONVERTED`   | Format changed (UUID preserved)                    |
| `MILESTONE`   | Significant version labeled                        |
| `RESTORED`    | Document reverted to a prior version               |

---

## 5. Collaboration Object

```json
"collaboration": {
  "pen-holder": "ian@firm.com",
  "markups": [
    {
      "id":               "mkp-001",
      "from":             "taxcounsel@partnerlaw.com",
      "from-display":     "Smith & Jones LLP",
      "submitted":        "2026-05-03T11:00:00Z",
      "base-version":     1,
      "base-fingerprint": "sha256:e3b0c442...",
      "status":           "incorporated",
      "incorporated-in":  2,
      "note":             "Tax review — payment terms and governing law",
      "ops": [ ... ]
    }
  ],
  "signoffs": [
    {
      "id":                     "so-001",
      "by":                     "taxcounsel@partnerlaw.com",
      "by-display":             "Smith & Jones LLP",
      "timestamp":              "2026-05-03T15:00:00Z",
      "version":                2,
      "fingerprint-at-signoff": "sha256:a87ff679...",
      "scope":                  ["Section 4", "Section 8", "Section 12"]
    }
  ]
}
```

Full markup and sign-off schemas are defined in FLP-0003.

---

## 6. Complete Minimal Record

The smallest valid Folio record — a document with one version, no 
collaboration:

```json
{
  "folio": "1.0",
  "identity": {
    "id":         "urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08",
    "created":    "2026-05-03T10:00:00Z",
    "created-by": "ian@firm.com"
  },
  "history": [
    {
      "v":           1,
      "author":      "ian@firm.com",
      "timestamp":   "2026-05-03T10:00:00Z",
      "fingerprint": "sha256:e3b0c44298fc1c149afbf4c8996fb924...",
      "ast-version": "3.2.1"
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

## 7. JSON Schema

See `schema/folio-record.schema.json` for the machine-readable schema.

The schema enforces:
- `id` matches `urn:folio:[uuid-v4]` pattern
- `fingerprint` matches `sha256:[64 hex chars]` pattern  
- `v` values are sequential integers starting at 1
- All timestamps are valid ISO8601 UTC strings
- `status` values are constrained enums
