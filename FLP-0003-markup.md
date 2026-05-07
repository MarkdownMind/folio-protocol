# FLP-0003: Markup and Sign-off Model
**Version:** 1.0.0-draft  
**Status:** Draft  
**Depends on:** FLP-0000, FLP-0001, FLP-0002

---

## 1. Abstract

This spec defines the pen-holder authority model, the markup (proposed
changes) lifecycle, and the sign-off (approval) mechanism.

The model reflects the reality of professional document collaboration:
one person owns the document and is responsible for its coherence.
Specialists contribute bounded input within their domain. The pen-holder
decides what to incorporate and maintains final responsibility.

This model applies regardless of document format. A markup from a
LibreOffice user against a DOCX document is handled identically to
a markup from a Word user. The Folio record is format-agnostic.

---

## 2. Roles

### 2.1 Pen-holder
The document owner. Has exclusive authority to:
- Incorporate markups into a new version
- Decline markups
- Create milestones
- Restore prior versions
- Transfer pen-holder status

There is exactly one pen-holder per document at any time. The pen-holder
is recorded in `collaboration.pen-holder`.

### 2.2 Contributor
Any party who submits a markup. Contributors:
- MAY propose changes via markup
- MAY sign off on a specific version
- MUST NOT directly commit new versions
- MUST NOT incorporate their own markups

---

## 3. Markup Lifecycle

```
submitted → [pending] → incorporated
                     → declined
                     → expired
```

### 3.1 Pending
Submitted but not yet reviewed by the pen-holder.

### 3.2 Incorporated
The pen-holder accepted the markup (in whole or in part) and created
a new version. The markup record is retained for audit.
`incorporated-in` records the resulting version number.

### 3.3 Declined
The pen-holder explicitly rejected the markup.
`decline-reason` SHOULD be recorded.

### 3.4 Expired
A markup submitted against version N is automatically expired when
the base version is more than 2 versions behind current.
Implementations MAY configure this threshold.

---

## 4. Markup Schema

Markups are stored in `collaboration.markups[]` in the Folio record.

```json
{
  "id":               "mkp-001",
  "from":             "taxcounsel@partnerlaw.com",
  "from-display":     "Smith & Jones LLP — Tax",
  "submitted":        "2026-05-03T11:00:00Z",
  "base-version":     1,
  "base-fingerprint": "sha256:e3b0c44298fc...",
  "status":           "incorporated",
  "incorporated-in":  2,
  "note":             "Tax review — payment terms and governing law",
  "ops": [
    {
      "type": "modify",
      "loc":  "Section 4.2, paragraph 1",
      "old":  "payment due net-30 from invoice date",
      "new":  "payment due net-60 from invoice date"
    },
    {
      "type": "insert",
      "loc":  "Section 7, after paragraph 3",
      "new":  "Governing law shall be the State of Delaware..."
    }
  ]
}
```

### 4.1 Markup Fields

| Field              | Required | Description                                    |
|--------------------|----------|------------------------------------------------|
| `id`               | MUST     | Unique within this document                    |
| `from`             | MUST     | Contributor identifier (email preferred)       |
| `from-display`     | SHOULD   | Human-readable name or organization            |
| `submitted`        | MUST     | ISO8601 UTC timestamp                          |
| `base-version`     | MUST     | Version number the markup was based on         |
| `base-fingerprint` | MUST     | Fingerprint of base version content            |
| `status`           | MUST     | `pending`, `incorporated`, `declined`, `expired`|
| `incorporated-in`  | MAY      | Version where markup was incorporated          |
| `note`             | SHOULD   | Purpose of this markup                         |
| `decline-reason`   | MAY      | Reason for decline (when status=declined)      |
| `ops`              | SHOULD   | Redline operations per FLP-0002                |

---

## 5. Sign-off Schema

Sign-offs are stored in `collaboration.signoffs[]`.

```json
{
  "id":                     "so-001",
  "by":                     "taxcounsel@partnerlaw.com",
  "by-display":             "Smith & Jones LLP — Tax",
  "timestamp":              "2026-05-03T15:00:00Z",
  "version":                2,
  "fingerprint-at-signoff": "sha256:a87ff679a2f3e71d...",
  "scope":                  ["Section 4", "Section 8", "Section 12"]
}
```

### 5.1 Sign-off Fields

| Field                    | Required | Description                              |
|--------------------------|----------|------------------------------------------|
| `id`                     | MUST     | Unique within this document              |
| `by`                     | MUST     | Approver identifier                      |
| `by-display`             | SHOULD   | Human-readable name                      |
| `timestamp`              | MUST     | ISO8601 UTC                              |
| `version`                | MUST     | Version being signed off                 |
| `fingerprint-at-signoff` | MUST     | Content fingerprint at time of sign-off  |
| `scope`                  | SHOULD   | Document sections reviewed               |

---

## 6. Stale Sign-off Detection

A sign-off becomes STALE when content changes after it was recorded.

Detection algorithm:
1. Compare `fingerprint-at-signoff` against current document fingerprint
2. If different: check whether changed sections overlap with `scope`
3. If overlap (or no scope specified): flag as stale

Display to pen-holder:
```
⚠ Tax counsel signed off v2 on May 3.
  Section 4 has been modified since their sign-off.
  Their approval may not reflect the current document.
```

This is one of Folio's most valuable features — it answers "has anything
changed since they approved it?" automatically and cryptographically.

---

## 7. Dispute Detection and Resolution

When multiple markups are pending against the same base version,
conforming tools MUST check for disputes before incorporation.

A dispute exists when:
- Two markups contain ops with the same `loc` value, AND
- The ops produce incompatible results (different `new` values)

### 7.1 Dispute Display

```
⚠ DISPUTE — Section 8, paragraph 2

  Tax counsel proposes:
    "indemnification cap of $1,000,000"

  Employment counsel proposes:
    "indemnification cap shall not apply to employment claims"

  These changes cannot both be incorporated.
  Choose one, modify, or defer.
```

### 7.2 Resolution

The pen-holder MUST choose:
- **Accept one** — incorporate one markup's version at this location
- **Modify** — write a new version that differs from both proposals
- **Defer** — decline both at this location, preserve base text

The resolution MUST be noted when creating the new version record.

---

## 8. Cross-Format Markup Flow

Because the Folio record is format-agnostic, markup exchange works
across format boundaries:

```
Pen-holder sends contract.docx to counterparty
  → Counterparty opens in LibreOffice, edits, saves as contract.odt
  → Returns contract.odt to pen-holder
  → Pen-holder's tool reads markup from contract.odt's Folio record
  → Ops reference AST locations (format-neutral)
  → Pen-holder incorporates into new version of contract.docx
```

The markup ops reference semantic document locations, not
format-specific element paths. This makes cross-format markup
exchange a first-class capability of the protocol.
