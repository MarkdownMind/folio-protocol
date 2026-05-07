// Package transport defines the Transport interface and all format adapters.
// Each adapter implements Read/Write for one document format.
// The protocol data model (core.Record) is identical across all transports.
package transport

import (
"archive/zip"
"bytes"
"encoding/json"
"fmt"
"io"
"os"
"path/filepath"
"strings"

"github.com/MarkdownMind/folio-protocol/internal/core"
)

// Transport is the interface all format adapters implement.
// Read returns nil, nil when no Folio record exists (not an error).
type Transport interface {
Read(documentPath string) (*core.Record, error)
Write(documentPath string, record *core.Record) error
Supports(ext string) bool
}

// For returns the appropriate transport adapter for a file extension.
// Falls back to SidecarTransport for any format without a native adapter.
func For(path string) Transport {
ext := strings.ToLower(filepath.Ext(path))
switch ext {
case ".folio":
		return &FolioTransport{}
	case ".docx":
return &DocxTransport{}
case ".odt", ".ods", ".odp":
return &ODFTransport{}
case ".pdf":
return &PDFTransport{}
default:
return &SidecarTransport{}
}
}

// ─── Sidecar Transport (FLP-0015) ────────────────────────────────────────────

type SidecarTransport struct{}

func (t *SidecarTransport) Supports(ext string) bool { return true }

func (t *SidecarTransport) sidecarPath(documentPath string) string {
return documentPath + ".folio"
}

func (t *SidecarTransport) Read(documentPath string) (*core.Record, error) {
sidecar := t.sidecarPath(documentPath)

data, err := os.ReadFile(sidecar)
if os.IsNotExist(err) {
return nil, nil
}
if err != nil {
return nil, fmt.Errorf("folio: read sidecar %s: %w", sidecar, err)
}

record, err := core.FromJSON(data)
if err != nil {
return nil, fmt.Errorf("folio: parse sidecar %s: %w", sidecar, err)
}

return record, nil
}

func (t *SidecarTransport) Write(documentPath string, record *core.Record) error {
sidecar := t.sidecarPath(documentPath)

data, err := record.ToJSON()
if err != nil {
return fmt.Errorf("folio: marshal record: %w", err)
}

if err := os.WriteFile(sidecar, data, 0644); err != nil {
return fmt.Errorf("folio: write sidecar %s: %w", sidecar, err)
}

return nil
}

// ─── DOCX Transport (FLP-0010) ───────────────────────────────────────────────

type DocxTransport struct{}

func (t *DocxTransport) Supports(ext string) bool { return ext == ".docx" }

const (
docxFolioPath   = "customXml/item_folio.json"
docxContentType = "application/vnd.folioprotocol+json"
)

func (t *DocxTransport) Read(documentPath string) (*core.Record, error) {
return readFromZIP(documentPath, docxFolioPath)
}

func (t *DocxTransport) Write(documentPath string, record *core.Record) error {
data, err := record.ToJSON()
if err != nil {
return fmt.Errorf("folio: marshal record: %w", err)
}

if len(data) > 900*1024 {
return fmt.Errorf(
"folio: record size %d bytes exceeds 900KB Word Online limit. "+
"Run compaction before saving.", len(data),
)
}

return writeToZIP(documentPath, docxFolioPath, data)
}

// ─── ODF Transport (FLP-0011) ────────────────────────────────────────────────

type ODFTransport struct{}

func (t *ODFTransport) Supports(ext string) bool {
return ext == ".odt" || ext == ".ods" || ext == ".odp"
}

const odfFolioPath = "META-INF/folio.json"

func (t *ODFTransport) Read(documentPath string) (*core.Record, error) {
return readFromZIP(documentPath, odfFolioPath)
}

func (t *ODFTransport) Write(documentPath string, record *core.Record) error {
data, err := record.ToJSON()
if err != nil {
return fmt.Errorf("folio: marshal record: %w", err)
}
return writeToZIP(documentPath, odfFolioPath, data)
}

// ─── PDF Transport (FLP-0012) ─────────────────────────────────────────────────

type PDFTransport struct {
fallback SidecarTransport
}

func (t *PDFTransport) Supports(ext string) bool { return ext == ".pdf" }

func (t *PDFTransport) Read(documentPath string) (*core.Record, error) {
// TODO: implement PDF attachment extraction using pdfcpu
return t.fallback.Read(documentPath)
}

func (t *PDFTransport) Write(documentPath string, record *core.Record) error {
// TODO: implement PDF attachment embedding using pdfcpu
return t.fallback.Write(documentPath, record)
}

// ─── ZIP helpers ──────────────────────────────────────────────────────────────

func readFromZIP(documentPath, internalPath string) (*core.Record, error) {
r, err := zip.OpenReader(documentPath)
if err != nil {
return nil, fmt.Errorf("folio: open zip %s: %w", documentPath, err)
}
defer r.Close()

for _, f := range r.File {
if f.Name == internalPath {
rc, err := f.Open()
if err != nil {
return nil, fmt.Errorf("folio: open %s in zip: %w", internalPath, err)
}
defer rc.Close()

data, err := io.ReadAll(rc)
if err != nil {
return nil, fmt.Errorf("folio: read %s from zip: %w", internalPath, err)
}

return core.FromJSON(data)
}
}

return nil, nil
}

func writeToZIP(documentPath, internalPath string, content []byte) error {
r, err := zip.OpenReader(documentPath)
if err != nil {
return fmt.Errorf("folio: open zip %s: %w", documentPath, err)
}

var buf bytes.Buffer
w := zip.NewWriter(&buf)

for _, f := range r.File {
if f.Name == internalPath {
continue
}

fw, err := w.CreateHeader(&f.FileHeader)
if err != nil {
r.Close()
return fmt.Errorf("folio: create zip entry %s: %w", f.Name, err)
}

rc, err := f.Open()
if err != nil {
r.Close()
return fmt.Errorf("folio: read zip entry %s: %w", f.Name, err)
}

if _, err := io.Copy(fw, rc); err != nil {
rc.Close()
r.Close()
return fmt.Errorf("folio: copy zip entry %s: %w", f.Name, err)
}
rc.Close()
}

r.Close()

fw, err := w.Create(internalPath)
if err != nil {
return fmt.Errorf("folio: create folio entry in zip: %w", err)
}
if _, err := fw.Write(content); err != nil {
return fmt.Errorf("folio: write folio entry: %w", err)
}

if err := w.Close(); err != nil {
return fmt.Errorf("folio: close zip writer: %w", err)
}

tmp := documentPath + ".folio.tmp"
if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
return fmt.Errorf("folio: write temp file: %w", err)
}
if err := os.Rename(tmp, documentPath); err != nil {
os.Remove(tmp)
return fmt.Errorf("folio: replace document file: %w", err)
}

return nil
}


// ─── Folio Transport ──────────────────────────────────────────────────────────
// Reads and writes standalone .folio JSON files (used as test corpus / sidecar).

type FolioTransport struct{}

func (t *FolioTransport) Supports(ext string) bool { return ext == ".folio" }

func (t *FolioTransport) Read(documentPath string) (*core.Record, error) {
data, err := os.ReadFile(documentPath)
if err != nil {
return nil, fmt.Errorf("folio: read %s: %w", documentPath, err)
}

// Strip _comment fields before parsing (used in test corpus only)
var raw map[string]json.RawMessage
if err := json.Unmarshal(data, &raw); err != nil {
return nil, fmt.Errorf("folio: parse %s: %w", documentPath, err)
}
delete(raw, "_comment")
cleaned, err := json.Marshal(raw)
if err != nil {
return nil, fmt.Errorf("folio: re-marshal %s: %w", documentPath, err)
}

return core.FromJSON(cleaned)
}

func (t *FolioTransport) Write(documentPath string, record *core.Record) error {
data, err := record.ToJSON()
if err != nil {
return fmt.Errorf("folio: marshal record: %w", err)
}
return os.WriteFile(documentPath, data, 0644)
}
