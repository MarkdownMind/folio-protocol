# FLP-0012: PDF Transport (XMP)

**Status:** Draft  
**Depends on:** FLP-0000, FLP-0001  
**Format:** Portable Document Format (PDF) — ISO 32000-2  

---

## Overview

PDF files contain an XMP (Extensible Metadata Platform) stream in the
document catalog. XMP supports arbitrary custom namespaces. Folio embeds
the Folio record inside the XMP stream as a custom namespace property.

This is the only standards-conforming way to embed arbitrary metadata
in a PDF without modifying the visible content or breaking print fidelity.

---

## XMP Namespace

```
URI:    http://schemas.folioprotocol.io/v1/
Prefix: folio
```

---

## XMP Embedding Format

The Folio record is JSON-encoded and stored as a single `folio:Record`
property within the document catalog's XMP metadata stream.

```xml
<?xpacket begin="﻿" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:folio="http://schemas.folioprotocol.io/v1/"
      xmlns:dc="http://purl.org/dc/elements/1.1/"
      xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
      <folio:Record><![CDATA[
        {
          "identity": { ... },
          "history": [ ... ],
          "events": [ ... ]
        }
      ]]></folio:Record>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
```

---

## Reading via pdfcpu (Go)

```go
import "github.com/pdfcpu/pdfcpu/pkg/api"

func readFolioPDF(path string) ([]byte, error) {
    xmp, err := api.ExtractMetadata(path)
    if err != nil {
        return nil, err
    }
    // Parse XMP and extract folio:Record CDATA content
    return extractFolioFromXMP(xmp), nil
}
```

---

## Reading via PyPDF (Python)

```python
from pypdf import PdfReader
import xml.etree.ElementTree as ET, json

def read_folio_pdf(path: str) -> dict | None:
    reader = PdfReader(path)
    xmp = reader.xmp_metadata
    if xmp is None:
        return None
    folio_ns = "http://schemas.folioprotocol.io/v1/"
    record_el = xmp.rdf_root.find(
        f".//{{{folio_ns}}}Record"
    )
    if record_el is None or record_el.text is None:
        return None
    return json.loads(record_el.text.strip())
```

---

## Implementation Status

The current `folio-go` implementation uses the **sidecar fallback**
for PDF files. Full XMP read/write requires pdfcpu or a similar library.

| Operation | Status              | Notes                              |
|-----------|---------------------|------------------------------------|
| Read      | Sidecar fallback    | `.folio` sidecar adjacent to PDF  |
| Write     | Sidecar fallback    | Creates `.folio` sidecar          |
| XMP read  | Planned (Phase 2)   | Requires pdfcpu integration        |
| XMP write | Planned (Phase 2)   | Requires pdfcpu integration        |

Until Phase 2, PDF documents use sidecar files automatically via
FLP-0015. This is transparent to the CLI user.

---

## Adobe Acrobat Preservation

Adobe Acrobat preserves XMP custom namespaces on save. The `folio:Record`
property survives routine editing and saving in Acrobat.

**Warning:** Print-to-PDF (macOS, Windows, any platform) creates a new
PDF without XMP metadata. If a user prints a tracked PDF to create a new
PDF, the Folio record is not carried. Use `folio convert` instead.

---

## Size Considerations

PDF XMP streams are typically stored in the document catalog, not in a
content stream. There is no hard limit in ISO 32000-2, but large XMP
streams can slow PDF readers on open. The FLP-0005 900 KB soft cap
applies. Compact the history log before embedding in PDF.
