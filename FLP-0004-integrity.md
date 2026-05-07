# FLP-0004: Cryptographic Integrity Model
**Version:** 1.0.0-draft  
**Status:** Draft  
**Depends on:** FLP-0000, FLP-0001

---

## 1. Abstract

Folio fingerprints operate on the **Pandoc AST JSON representation** of 
document content — not on the raw format-specific bytes. This makes 
fingerprints format-agnostic: the same document content in DOCX and ODT 
produces the same fingerprint.

The tradeoff is explicit: formatting differences (margins, fonts, colors) 
do not affect the fingerprint. Content differences (text, structure, 
tables, lists) do. This is the correct behavior for a protocol focused 
on document meaning rather than document appearance.

---

## 2. The Fingerprint Algorithm

```
fingerprint = "sha256:" + hex(SHA256(canonicalize(pandoc_ast_json(document))))
```

In full:

```
1. Convert document to Pandoc AST JSON
   pandoc {input-file} -t json -o {output.json}

2. Canonicalize the AST JSON (Section 3)

3. SHA-256 hash the UTF-8 bytes of the canonical JSON

4. Hex-encode, prepend "sha256:"
```

---

## 3. AST Canonicalization

The Pandoc AST JSON must be canonicalized before hashing to ensure 
identical content always produces identical fingerprints regardless of 
minor JSON serialization differences.

### 3.1 Strip the pandoc-api-version field

The `pandoc-api-version` field changes between Pandoc releases. Strip 
it before hashing. It is recorded separately in the version record as 
`ast-version` for reproducibility.

```python
ast = json.loads(pandoc_json_output)
api_version = ast.pop("pandoc-api-version", None)  # record separately
```

### 3.2 Strip metadata noise

Remove fields that change without semantic content change:

```python
# Remove from meta object:
noise_meta_fields = [
    "date",           # document date metadata (not content)
    "generator",      # software that created the file  
    "producer",       # PDF producer field
    "modified",       # last-modified timestamp
    "revision",       # ODF revision counter
]
```

### 3.3 Normalize whitespace in text nodes

Collapse multiple consecutive spaces in `Str` nodes to single spaces.
Trim leading/trailing whitespace from `Str` nodes.

### 3.4 Sort metadata keys

JSON object key order is not guaranteed. Sort all object keys 
alphabetically before hashing to ensure determinism.

### 3.5 Compact serialization

Serialize as compact JSON (no indentation, no trailing whitespace):

```python
canonical = json.dumps(ast, sort_keys=True, separators=(',', ':'), 
                        ensure_ascii=False)
```

---

## 4. Reference Implementation (Python)

```python
import json
import hashlib
import subprocess
import tempfile
import os

def compute_fingerprint(document_path: str) -> tuple[str, str]:
    """
    Compute a Folio fingerprint for any document supported by Pandoc.
    
    Returns (fingerprint, ast_version) tuple.
    
    fingerprint: "sha256:" + 64 hex chars
    ast_version: Pandoc API version string (e.g. "3.2.1")
    """
    # Step 1: Convert to Pandoc AST JSON
    result = subprocess.run(
        ["pandoc", document_path, "-t", "json"],
        capture_output=True, text=True, check=True
    )
    
    ast = json.loads(result.stdout)
    
    # Step 2: Extract and record API version
    api_version_parts = ast.pop("pandoc-api-version", [0, 0, 0])
    ast_version = ".".join(str(x) for x in api_version_parts)
    
    # Step 3: Strip metadata noise
    _strip_meta_noise(ast)
    
    # Step 4: Normalize whitespace in text nodes
    _normalize_text_nodes(ast)
    
    # Step 5: Canonical JSON serialization
    canonical = json.dumps(
        ast, 
        sort_keys=True, 
        separators=(',', ':'),
        ensure_ascii=False
    )
    
    # Step 6: SHA-256
    digest = hashlib.sha256(canonical.encode('utf-8')).hexdigest()
    fingerprint = f"sha256:{digest}"
    
    return fingerprint, ast_version


NOISE_META_KEYS = {
    "date", "generator", "producer", "modified", 
    "revision", "editing-cycles", "creation-date"
}

def _strip_meta_noise(ast: dict) -> None:
    """Remove metadata fields that change without semantic content change."""
    meta = ast.get("meta", {})
    for key in list(meta.keys()):
        if key in NOISE_META_KEYS:
            del meta[key]


def _normalize_text_nodes(node) -> None:
    """
    Walk the AST and normalize whitespace in Str nodes.
    Pandoc AST nodes are {'t': 'TypeName', 'c': content}
    """
    if isinstance(node, dict):
        if node.get("t") == "Str" and isinstance(node.get("c"), str):
            # Normalize whitespace
            node["c"] = " ".join(node["c"].split())
        else:
            for value in node.values():
                _normalize_text_nodes(value)
    elif isinstance(node, list):
        for item in node:
            _normalize_text_nodes(item)


def verify_fingerprint(document_path: str, stored_fingerprint: str, 
                        stored_ast_version: str) -> bool:
    """
    Verify document content against a stored fingerprint.
    
    Note: If the current Pandoc version differs from stored_ast_version,
    the fingerprint MAY not match even if content is identical.
    Implementations SHOULD warn when Pandoc versions differ.
    """
    current_fp, current_version = compute_fingerprint(document_path)
    
    if current_version != stored_ast_version:
        import warnings
        warnings.warn(
            f"Pandoc AST version mismatch: stored={stored_ast_version}, "
            f"current={current_version}. Fingerprint comparison may be "
            f"unreliable. See FLP-0004 §5."
        )
    
    return current_fp == stored_fingerprint
```

---

## 5. The Pandoc Version Problem

The Pandoc AST JSON format evolves between major versions. A fingerprint 
computed with Pandoc 3.1 may not match the same document fingerprinted 
with Pandoc 3.3, even if content is identical, because the AST 
representation may differ.

**Mitigation:**

1. Record `ast-version` in every version record (FLP-0001 §3.1)
2. When verifying, warn if current Pandoc version differs from stored
3. Implementations SHOULD maintain a Pandoc version pinned per 
   installation and document when that version changes
4. A future FLP-0004 revision will define a stable AST subset that 
   is guaranteed stable across Pandoc versions

**For now:** The `ast-version` field provides enough information to 
detect potential mismatches and reproduce fingerprints using the 
original Pandoc version.

---

## 6. Format-Specific Notes

### 6.1 DOCX

Pandoc's DOCX reader handles most Word documents well. Known limitations:
- Complex table merging may be simplified in the AST
- Some field codes are not preserved in the AST
- Page layout information (margins, headers) is not in the AST

These are acceptable because Folio fingerprints content, not layout.

### 6.2 ODT / ODF

Pandoc's ODT reader is mature. ODF documents generally round-trip 
through Pandoc with high fidelity for text content.

### 6.3 PDF

PDF support in Pandoc is read-only and limited — it extracts text 
but loses structure. For PDF fingerprinting, Folio uses a different 
approach: hash the PDF content streams directly after stripping 
volatile metadata (creation date, producer, mod date).

```python
def compute_pdf_fingerprint(pdf_path: str) -> tuple[str, str]:
    """
    PDF-specific fingerprinting: hash content streams, not AST.
    Returns (fingerprint, "pdf-streams-v1") tuple.
    """
    import pikepdf  # pip install pikepdf
    
    with pikepdf.open(pdf_path) as pdf:
        content_data = bytearray()
        for page in pdf.pages:
            # Extract raw content streams (rendering instructions)
            # These change only when visual content changes
            for key in ['/Contents']:
                if key in page:
                    obj = page[key]
                    if hasattr(obj, 'read_bytes'):
                        content_data.extend(obj.read_bytes())
    
    digest = hashlib.sha256(bytes(content_data)).hexdigest()
    return f"sha256:{digest}", "pdf-streams-v1"
```

### 6.4 Plain text / Markdown

For plain text formats, Pandoc AST is used directly. These formats 
have minimal noise and high fingerprint stability.

---

## 7. Format-Agnostic Verification

A key property: if the same content is in DOCX and ODT, both produce 
the same fingerprint (modulo the Pandoc version and table complexity 
caveats above).

This means a CONVERTED event can be verified:
1. Fingerprint the DOCX at conversion time
2. Fingerprint the resulting ODT using the same Pandoc version
3. If fingerprints match: content was preserved through conversion
4. If they differ: some content was lost or transformed during conversion

This makes format conversion auditable — something no other protocol 
currently provides.
