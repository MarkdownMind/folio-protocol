# FLP-0010: DOCX Transport

**Status:** Draft  
**Depends on:** FLP-0000, FLP-0001  
**Format:** Office Open XML (DOCX) — ISO/IEC 29500  

---

## Overview

The DOCX transport embeds the Folio record inside the DOCX ZIP container
as a Custom XML Part. The record is JSON, not XML — the Custom XML Part
mechanism supports arbitrary content types per ISO/IEC 29500 Part 2.

Word and compatible applications ignore unknown Custom XML Parts. The
document opens and edits normally whether or not Folio is installed.

---

## Embedding Location

```
document.docx (ZIP container)
└── customXml/
    ├── item_folio.json        ← Folio record (UTF-8 JSON)
    └── itemProps_folio.xml    ← content type declaration
```

**Internal path:** `customXml/item_folio.json`  
**Content-Type:** `application/vnd.folioprotocol+json`

---

## Content_Types Registration

Add to `[Content_Types].xml`:

```xml
<Override PartName="/customXml/item_folio.json"
          ContentType="application/vnd.folioprotocol+json"/>
```

---

## itemProps Declaration

```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<ds:datastoreItem ds:itemID="{FOLIO-ITEM-ID}"
  xmlns:ds="http://schemas.openxmlformats.org/officeDocument/2006/customXml">
  <ds:schemaRefs>
    <ds:schemaRef ds:uri="http://schemas.folioprotocol.io/v1"/>
  </ds:schemaRefs>
</ds:datastoreItem>
```

---

## Access via Office.js

```javascript
// Read
Office.context.document.customXmlParts
  .getByNamespaceAsync("http://schemas.folioprotocol.io/v1",
    (result) => {
      if (result.value.length === 0) return; // not tracking
      result.value[0].getXmlAsync((xmlResult) => {
        const record = JSON.parse(xmlResult.value);
      });
    });

// Write
Office.context.document.customXmlParts
  .addAsync(JSON.stringify(folioRecord), { asyncContext: null }, callback);
```

---

## Access via Open XML SDK (C#)

```csharp
using DocumentFormat.OpenXml.Packaging;
using System.Linq;

// Read
using var doc = WordprocessingDocument.Open(path, false);
var part = doc.MainDocumentPart?.CustomXmlParts
    .FirstOrDefault(p => p.ContentType == "application/vnd.folioprotocol+json");

if (part != null) {
    using var stream = part.GetStream();
    var record = JsonSerializer.Deserialize<FolioRecord>(stream);
}

// Write
using var doc = WordprocessingDocument.Open(path, true);
var newPart = doc.MainDocumentPart
    .AddCustomXmlPart("application/vnd.folioprotocol+json");
using var stream = newPart.GetStream(FileMode.Create);
JsonSerializer.Serialize(stream, folioRecord);
```

---

## Access via Go (folio-go)

```go
// Read
r, _ := zip.OpenReader("document.docx")
for _, f := range r.File {
    if f.Name == "customXml/item_folio.json" {
        data, _ := io.ReadAll(f.Open())
        record, _ := core.FromJSON(data)
    }
}
```

See `internal/transport/transport.go` for the full implementation.

---

## Size Limits

| Limit       | Value  | Source                             |
|-------------|--------|------------------------------------|
| Hard limit  | 1 MB   | Word Online CustomXmlParts bug     |
| Soft cap    | 900 KB | FLP-0005 §3.1 — trigger compaction |
| Recommended | 750 KB | Compact before approaching cap     |

Reference: [OfficeDev/office-js#2408](https://github.com/OfficeDev/office-js/issues/2408)

---

## Known Issues

**Word Online write instability:** CustomXmlParts writes may fail silently
in Word Online. Implementations MUST:
1. Verify the write succeeded by reading back immediately.
2. Degrade to read-only mode if writes fail consistently.
3. Inform the user that changes are not being tracked.

Reference: [OfficeDev/office-js#5910](https://github.com/OfficeDev/office-js/issues/5910)

---

## Compatibility

| Application      | Read | Write | Notes                        |
|-----------------|------|-------|------------------------------|
| Word desktop    | ✓    | ✓     | Full support                 |
| Word Online     | ✓    | ⚠     | Write may fail (see above)   |
| LibreOffice     | ✓    | ✓     | Opens DOCX; ignores part     |
| Google Docs     | ✗    | ✗     | Strips custom XML on import  |

Google Docs strips Custom XML Parts on import. If a DOCX enters Google
Docs, the Folio record is lost. Use the sidecar transport (FLP-0015)
for documents expected to pass through Google Docs.
