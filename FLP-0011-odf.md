# FLP-0011: ODF Transport (ODT, ODS, ODP, EPUB)

**Status:** Draft  
**Depends on:** FLP-0000, FLP-0001  
**Formats:** ODF (ODT/ODS/ODP) — ISO/IEC 26300; EPUB — OCF 3.2  

---

## Overview

ODF documents and EPUB books both use the OCF ZIP container format.
Both allow additional files in `META-INF/` that conforming processors
MUST ignore if they do not recognize them.

Folio uses this clean injection point identically for both formats.
A single adapter implementation covers all four file types.

---

## Embedding Location

```
document.odt (ZIP)
└── META-INF/
    ├── manifest.xml           ← standard ODF manifest
    └── folio.json             ← Folio record (UTF-8 JSON)

book.epub (ZIP)
└── META-INF/
    ├── container.xml          ← standard EPUB container
    └── folio.json             ← Folio record (UTF-8 JSON)
```

**Internal path:** `META-INF/folio.json`  
**Content-Type:** `application/vnd.folioprotocol+json`

---

## ODF Manifest Entry (recommended)

Per ODF Package Specification (ISO/IEC 26300-3 §4.3), files in
`META-INF/` do not require manifest entries. Adding one is RECOMMENDED
for discoverability but NOT REQUIRED for conformance.

```xml
<manifest:file-entry
  manifest:media-type="application/vnd.folioprotocol+json"
  manifest:full-path="META-INF/folio.json"/>
```

---

## EPUB Note

EPUB OCF 3.2 §3.5.1 states:

> "Reading Systems MUST ignore any unknown files in the META-INF directory."

This makes `META-INF/folio.json` a guaranteed safe location in EPUB.
No format changes, no manifest registration, no reading system breakage.

---

## Access via Python

```python
import zipfile, json

def read_folio(path: str) -> dict | None:
    with zipfile.ZipFile(path) as zf:
        try:
            with zf.open("META-INF/folio.json") as f:
                return json.load(f)
        except KeyError:
            return None  # no Folio record

def write_folio(path: str, record: dict) -> None:
    import os, tempfile
    tmp = path + ".folio.tmp"
    with zipfile.ZipFile(path) as zin:
        with zipfile.ZipFile(tmp, 'w', zipfile.ZIP_DEFLATED) as zout:
            for item in zin.infolist():
                if item.filename != "META-INF/folio.json":
                    zout.writestr(item, zin.read(item.filename))
            zout.writestr(
                "META-INF/folio.json",
                json.dumps(record, indent=2, ensure_ascii=False)
            )
    os.replace(tmp, path)
```

---

## Access via Go (folio-go)

```go
// Read
record, err := transport.For("document.odt").Read("document.odt")

// Write
err = transport.For("document.odt").Write("document.odt", record)
```

See `internal/transport/transport.go` for the full implementation.

---

## LibreOffice Preservation

LibreOffice preserves unknown `META-INF/` files on save in all ODF
formats. Tested with LibreOffice 7.x and 24.x on Linux, macOS, Windows.

**Warning:** LibreOffice's DOCX round-trip (`.odt` → save as `.docx`)
does not preserve `META-INF/` data, because the target format (DOCX)
uses a different container structure. If users save ODT as DOCX via
LibreOffice, use `folio convert` instead to preserve the record.

---

## Applies To

| Extension | Format      | Spec          |
|-----------|-------------|---------------|
| `.odt`    | ODF Text    | ISO 26300     |
| `.ods`    | ODF Spreadsheet | ISO 26300 |
| `.odp`    | ODF Presentation | ISO 26300|
| `.epub`   | EPUB 3.x    | OCF 3.2       |
