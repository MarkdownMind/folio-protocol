# FLP-0010: DOCX Transport
**Depends on:** FLP-0000, FLP-0001

## Embedding Location

The Folio record is embedded in the DOCX ZIP container as a Custom XML 
Part, using the mechanism defined in ISO/IEC 29500 Part 2.

```
document.docx (ZIP)
└── customXml/
    ├── item_folio.json        ← Folio record (JSON, not XML)
    └── itemProps_folio.xml    ← namespace declaration
```

Note: Unlike the previous v1 spec, the Folio record is JSON (not XML) 
for consistency across all transports. The Custom XML Part mechanism 
supports any content type.

## Registration

Add to `[Content_Types].xml`:
```xml
<Override PartName="/customXml/item_folio.json"
          ContentType="application/vnd.folioprotocol+json"/>
```

## Access via Office.js

```javascript
// Read
Office.context.document.customXmlParts
  .getByNamespaceAsync("http://schemas.folioprotocol.io/v1",
    (result) => { /* parse result.value[0] */ });

// Write  
Office.context.document.customXmlParts
  .addAsync(JSON.stringify(folioRecord), callback);
```

## Access via Open XML SDK (C#)

```csharp
using DocumentFormat.OpenXml.Packaging;

// Read
var part = doc.MainDocumentPart.CustomXmlParts
    .FirstOrDefault(p => /* check content type */);

// Write
var newPart = doc.MainDocumentPart
    .AddCustomXmlPart(CustomXmlPartType.CustomXml);
using var stream = newPart.GetStream(FileMode.Create);
JsonSerializer.Serialize(stream, folioRecord);
```

## Constraints

- Part size MUST NOT exceed 900KB (Word Online hard limit)
- Compaction required at 750KB (see FLP-0005)
- Word Online write operations may be unreliable (see FLP-0005 §2.1)

---

# FLP-0011: ODT/ODF Transport
**Depends on:** FLP-0000, FLP-0001

## Embedding Location

ODF is also a ZIP container. The Folio record lives in `META-INF/`:

```
document.odt (ZIP)
└── META-INF/
    ├── manifest.xml           ← standard ODF manifest (updated)
    └── folio.json             ← Folio record
```

Per ODF Package Specification (ISO/IEC 26300-3), files in `META-INF/` 
do not require manifest entries and must be ignored by non-conforming 
applications. LibreOffice preserves unknown `META-INF/` files on save.

## Manifest Entry (optional but recommended)

```xml
<manifest:file-entry 
  manifest:media-type="application/vnd.folioprotocol+json"
  manifest:full-path="META-INF/folio.json"/>
```

Per ODF validation rules, the manifest need not contain entries for 
files in `META-INF/`. The entry is RECOMMENDED for discoverability 
but not required for conformance.

## Access (Python with odfpy)

```python
from odf.opendocument import load
import zipfile, json

def read_folio_odt(path: str) -> dict | None:
    with zipfile.ZipFile(path) as zf:
        try:
            with zf.open("META-INF/folio.json") as f:
                return json.load(f)
        except KeyError:
            return None  # no Folio record

def write_folio_odt(path: str, record: dict) -> None:
    import shutil, tempfile, os
    tmp = path + ".tmp"
    with zipfile.ZipFile(path) as zin:
        with zipfile.ZipFile(tmp, 'w', zipfile.ZIP_DEFLATED) as zout:
            for item in zin.infolist():
                if item.filename != "META-INF/folio.json":
                    zout.writestr(item, zin.read(item.filename))
            zout.writestr("META-INF/folio.json", 
                         json.dumps(record, ensure_ascii=False))
    os.replace(tmp, path)
```

## EPUB Note

EPUB uses the same OCF container format as ODF. The EPUB OCF 3.2 spec 
explicitly states that `META-INF/` may contain additional files and 
processors MUST NOT fail on encountering them. The FLP-0011 transport 
applies to EPUB without modification.

---

# FLP-0012: PDF Transport (XMP)
**Depends on:** FLP-0000, FLP-0001

## Embedding Mechanism

PDF supports XMP (ISO 16684-1:2019) — an XML-based metadata format 
embedded in the PDF file. XMP supports custom namespaces. Folio 
embeds the record as a custom XMP property set.

## XMP Namespace

```
URI:    http://schemas.folioprotocol.io/v1/xmp/
Prefix: folio
```

## XMP Structure

```xml
<?xpacket begin='' id='W5M0MpCehiHzreSzNTczkc9d'?>
<x:xmpmeta xmlns:x='adobe:ns:meta/'>
  <rdf:RDF xmlns:rdf='http://www.w3.org/1999/02/22-rdf-syntax-ns#'>
    <rdf:Description rdf:about=''
      xmlns:folio='http://schemas.folioprotocol.io/v1/xmp/'>
      
      <folio:id>urn:folio:a3f9c821-4e2d-4b8c-9f1a-7d6e3c2b5a08</folio:id>
      <folio:version>3</folio:version>
      <folio:fingerprint>sha256:e3b0c44298fc...</folio:fingerprint>
      <folio:author>ian@firm.com</folio:author>
      <folio:timestamp>2026-05-04T09:00:00Z</folio:timestamp>
      <folio:record-uri>folio:embedded</folio:record-uri>
      
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end='w'?>
```

The XMP stores summary fields for quick access. The complete Folio 
record JSON is embedded as an attachment stream in the PDF:

## Full Record as PDF Attachment

```python
import pikepdf
import json

def embed_folio_pdf(pdf_path: str, record: dict) -> None:
    with pikepdf.open(pdf_path, allow_overwriting_input=True) as pdf:
        # Embed as file attachment
        record_bytes = json.dumps(record, ensure_ascii=False).encode('utf-8')
        
        pdf.attachments['folio.json'] = pikepdf.AttachedFileSpec(
            pdf, 
            record_bytes,
            description="Folio Protocol Record",
            filename="folio.json",
            mime_type="application/vnd.folioprotocol+json"
        )
        
        # Update XMP summary
        _update_xmp(pdf, record)
        pdf.save()
```

## PDF Fingerprinting

PDF fingerprints use content streams, not Pandoc AST (see FLP-0004 §6.3).

---

# FLP-0015: Sidecar Transport
**Depends on:** FLP-0000, FLP-0001

## Overview

The sidecar is the universal fallback transport. It works for any 
document format and any situation where embedded metadata is undesirable.

## Naming Convention

```
{document-filename}.folio
```

Examples:
```
contract.docx       → contract.docx.folio
brief.pdf           → brief.pdf.folio
notes.txt           → notes.txt.folio
research.md         → research.md.folio
data.csv            → data.csv.folio
```

## Format

The `.folio` file is plain JSON, UTF-8 encoded, human-readable:

```json
{
  "folio": "1.0",
  "sidecar-for": "contract.docx",
  "identity": {
    "id": "urn:folio:a3f9c821-...",
    "created": "2026-05-03T10:00:00Z",
    "created-by": "ian@firm.com"
  },
  "history": [ ... ],
  "collaboration": { ... }
}
```

## Priority Rule

When both an embedded record and a sidecar exist for the same document, 
the embedded record takes precedence. Tools SHOULD warn if they conflict.

## Git Integration

Sidecar files commit naturally to git alongside documents:

```
contract.docx
contract.docx.folio
research.md
research.md.folio
```

The `.folio` files are small JSON, diff cleanly, and provide meaningful 
`git diff` output — the version history as a readable JSON log.

## .gitignore Note

Users who do NOT want sidecar files committed should add to `.gitignore`:
```
*.folio
```

Users who DO want them committed (recommended for audit purposes) do 
nothing — they commit naturally.
