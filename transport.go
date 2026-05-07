// Package transport defines the Transport interface and all format adapters.
// Each adapter implements Read/Write for one document format.
// The protocol data model (core.Record) is identical across all transports.
package transport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/folioprotocol/folio-go/internal/core"
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
// Universal fallback. Works for any document format.
// Creates {document}.folio adjacent to the document file.

type SidecarTransport struct{}

func (t *SidecarTransport) Supports(ext string) bool { return true }

func (t *SidecarTransport) sidecarPath(documentPath string) string {
	return documentPath + ".folio"
}

func (t *SidecarTransport) Read(documentPath string) (*core.Record, error) {
	sidecar := t.sidecarPath(documentPath)

	data, err := os.ReadFile(sidecar)
	if os.IsNotExist(err) {
		return nil, nil // no record — not an error
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
// Embeds the Folio record in the DOCX ZIP container as a Custom XML Part.
// Word ignores unknown custom XML parts per ISO/IEC 29500 Part 2.

type DocxTransport struct{}

func (t *DocxTransport) Supports(ext string) bool { return ext == ".docx" }

const (
	docxFolioPath    = "customXml/item_folio.json"
	docxContentType  = "application/vnd.folioprotocol+json"
)

func (t *DocxTransport) Read(documentPath string) (*core.Record, error) {
	return readFromZIP(documentPath, docxFolioPath)
}

func (t *DocxTransport) Write(documentPath string, record *core.Record) error {
	data, err := record.ToJSON()
	if err != nil {
		return fmt.Errorf("folio: marshal record: %w", err)
	}

	// Size check: Word Online hard limit is 1MB, we cap at 900KB (FLP-0005)
	if len(data) > 900*1024 {
		return fmt.Errorf(
			"folio: record size %d bytes exceeds 900KB Word Online limit. "+
				"Run compaction before saving.", len(data),
		)
	}

	return writeToZIP(documentPath, docxFolioPath, data)
}

// ─── ODF Transport (FLP-0011) ────────────────────────────────────────────────
// Embeds the Folio record in META-INF/folio.json inside the ODF ZIP container.
// ODF spec: files in META-INF/ do not require manifest entries.
// LibreOffice preserves unknown META-INF/ files on save.

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
// Embeds the Folio record as a JSON attachment in the PDF file.
// Full XMP custom namespace support is deferred to a future revision.
// This implementation uses PDF file attachments (universally supported).
//
// Production implementation should use pdfcpu (github.com/pdfcpu/pdfcpu).
// For the reference implementation, we fall back to sidecar.

type PDFTransport struct {
	fallback SidecarTransport
}

func (t *PDFTransport) Supports(ext string) bool { return ext == ".pdf" }

func (t *PDFTransport) Read(documentPath string) (*core.Record, error) {
	// TODO: implement PDF attachment extraction using pdfcpu
	// pdfcpu.ExtractAttachmentsFile(documentPath, outDir, []string{"folio.json"}, nil)
	// For now: fall back to sidecar
	return t.fallback.Read(documentPath)
}

func (t *PDFTransport) Write(documentPath string, record *core.Record) error {
	// TODO: implement PDF attachment embedding using pdfcpu
	// pdfcpu.AddAttachmentsFile(documentPath, "", []string{sidecarPath}, nil)
	// For now: fall back to sidecar
	return t.fallback.Write(documentPath, record)
}

// ─── ZIP helpers (used by DOCX and ODF) ──────────────────────────────────────

import (
	"archive/zip"
	"bytes"
	"io"
)

// readFromZIP reads a named file from inside a ZIP-based document (DOCX/ODF).
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

	return nil, nil // not found — no Folio record
}

// writeToZIP writes or replaces a named file inside a ZIP-based document.
// It rewrites the entire ZIP, replacing the target file if it exists.
func writeToZIP(documentPath, internalPath string, content []byte) error {
	// Read existing ZIP
	r, err := zip.OpenReader(documentPath)
	if err != nil {
		return fmt.Errorf("folio: open zip %s: %w", documentPath, err)
	}

	// Build new ZIP in memory
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	folioWritten := false

	for _, f := range r.File {
		// Skip the old Folio record — we'll write the new one
		if f.Name == internalPath {
			continue
		}

		// Copy all other files unchanged
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

	// Write the Folio record
	fw, err := w.Create(internalPath)
	if err != nil {
		return fmt.Errorf("folio: create folio entry in zip: %w", err)
	}
	if _, err := fw.Write(content); err != nil {
		return fmt.Errorf("folio: write folio entry: %w", err)
	}
	folioWritten = true
	_ = folioWritten

	if err := w.Close(); err != nil {
		return fmt.Errorf("folio: close zip writer: %w", err)
	}

	// Atomically replace the document file
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
