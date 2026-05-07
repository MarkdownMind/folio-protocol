// Package core defines the Folio Protocol data model.
// All types map directly to the JSON schema in folio-record.schema.json.
// See FLP-0001 for the full specification.
package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

const (
	ProtocolVersion = "1.0"
	Namespace       = "http://schemas.folioprotocol.io/v1"
)

// Valid fingerprint: "sha256:" + 64 lowercase hex chars
var fingerprintRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Valid Folio URN: urn:folio: + UUID v4
var folioUrnRe = regexp.MustCompile(
	`^urn:folio:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)

// ─── Identity ────────────────────────────────────────────────────────────────

// Identity is the permanent record of what a document is.
// The ID never changes across versions, renames, copies, or format conversions.
type Identity struct {
	ID        string   `json:"id"`                  // urn:folio:{uuid-v4}
	Title     string   `json:"title,omitempty"`
	Created   string   `json:"created"`             // ISO8601 UTC
	CreatedBy string   `json:"created-by"`
	Lineage   []string `json:"lineage,omitempty"`   // parent document IDs
}

// NewIdentity creates a new Identity with a fresh UUID v4.
func NewIdentity(author, title string) Identity {
	id := fmt.Sprintf("urn:folio:%s", uuid.New().String())
	return Identity{
		ID:        id,
		Title:     title,
		Created:   utcNow(),
		CreatedBy: author,
		Lineage:   []string{},
	}
}

// ─── Version ─────────────────────────────────────────────────────────────────

// Version is a single entry in the document history.
// The history is append-only. Version numbers are sequential from 1.
type Version struct {
	V           int    `json:"v"`                    // monotonically increasing from 1
	Author      string `json:"author"`
	Timestamp   string `json:"timestamp"`            // ISO8601 UTC
	Fingerprint string `json:"fingerprint"`          // sha256:{64 hex}
	ASTVersion  string `json:"ast-version"`          // pandoc API version
	Note        string `json:"note,omitempty"`
	Format      string `json:"format,omitempty"`     // docx, odt, pdf, etc.
}

// ─── Event ───────────────────────────────────────────────────────────────────

// EventType is the semantic type of a lifecycle event.
type EventType string

const (
	EventInitialized  EventType = "INITIALIZED"
	EventVersioned    EventType = "VERSIONED"
	EventSent         EventType = "SENT"
	EventMarkupAdded  EventType = "MARKUP_ADDED"
	EventIncorporated EventType = "INCORPORATED"
	EventSignedOff    EventType = "SIGNED_OFF"
	EventConverted    EventType = "CONVERTED"
	EventMilestone    EventType = "MILESTONE"
	EventRestored     EventType = "RESTORED"
)

// Event is a semantic lifecycle event in the document's history.
// Extra fields are stored in Data and round-trip through JSON.
type Event struct {
	Event     EventType              `json:"event"`
	Timestamp string                 `json:"timestamp"`
	By        string                 `json:"by"`
	Data      map[string]interface{} `json:"-"`
}

func (e Event) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"event":     e.Event,
		"timestamp": e.Timestamp,
		"by":        e.By,
	}
	for k, v := range e.Data {
		m[k] = v
	}
	return json.Marshal(m)
}

func (e *Event) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	e.Event = EventType(m["event"].(string))
	e.Timestamp, _ = m["timestamp"].(string)
	e.By, _ = m["by"].(string)
	e.Data = make(map[string]interface{})
	for k, v := range m {
		if k != "event" && k != "timestamp" && k != "by" {
			e.Data[k] = v
		}
	}
	return nil
}

// ─── Markup ───────────────────────────────────────────────────────────────────

// MarkupStatus is the lifecycle state of a proposed change.
type MarkupStatus string

const (
	MarkupPending      MarkupStatus = "pending"
	MarkupIncorporated MarkupStatus = "incorporated"
	MarkupDeclined     MarkupStatus = "declined"
	MarkupExpired      MarkupStatus = "expired"
)

// Op is a single redline operation per FLP-0002.
type Op struct {
	Type        string `json:"type"`                  // modify, insert, delete, format, move
	Loc         string `json:"loc,omitempty"`
	Old         string `json:"old,omitempty"`
	New         string `json:"new,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Description string `json:"description,omitempty"`
}

// Markup is a set of proposed changes from a contributor.
// Contributors propose; only the pen-holder incorporates.
type Markup struct {
	ID              string       `json:"id"`
	From            string       `json:"from"`
	FromDisplay     string       `json:"from-display,omitempty"`
	Submitted       string       `json:"submitted"`
	BaseVersion     int          `json:"base-version"`
	BaseFingerprint string       `json:"base-fingerprint"`
	Status          MarkupStatus `json:"status"`
	IncorporatedIn  int          `json:"incorporated-in,omitempty"`
	Note            string       `json:"note,omitempty"`
	DeclineReason   string       `json:"decline-reason,omitempty"`
	Ops             []Op         `json:"ops,omitempty"`
}

// ─── Sign-off ─────────────────────────────────────────────────────────────────

// Signoff records a reviewer's approval of a specific version.
// The FingerprintAtSignoff field anchors the approval to the exact content
// at the time of signing. If content changes after sign-off, the fingerprint
// will not match and the sign-off is flagged as stale.
type Signoff struct {
	ID                   string   `json:"id"`
	By                   string   `json:"by"`
	ByDisplay            string   `json:"by-display,omitempty"`
	Timestamp            string   `json:"timestamp"`
	Version              int      `json:"version"`
	FingerprintAtSignoff string   `json:"fingerprint-at-signoff"`
	Scope                []string `json:"scope,omitempty"`
}

// IsStale returns true if the document content has changed since sign-off.
func (s Signoff) IsStale(currentFingerprint string) bool {
	return s.FingerprintAtSignoff != currentFingerprint
}

// ─── Collaboration ────────────────────────────────────────────────────────────

// Collaboration holds the pen-holder model state.
type Collaboration struct {
	PenHolder string    `json:"pen-holder"`
	Markups   []Markup  `json:"markups"`
	Signoffs  []Signoff `json:"signoffs"`
}

// ─── Record ───────────────────────────────────────────────────────────────────

// Record is the complete Folio Protocol record for a document.
// This is the top-level JSON object embedded in or alongside any document.
type Record struct {
	Folio         string        `json:"folio"`
	Identity      Identity      `json:"identity"`
	History       []Version     `json:"history"`
	Events        []Event       `json:"events"`
	Collaboration Collaboration `json:"collaboration"`
}

// NewRecord creates a minimal valid Record.
func NewRecord(identity Identity, author string) *Record {
	return &Record{
		Folio:    ProtocolVersion,
		Identity: identity,
		History:  []Version{},
		Events:   []Event{},
		Collaboration: Collaboration{
			PenHolder: author,
			Markups:   []Markup{},
			Signoffs:  []Signoff{},
		},
	}
}

// CurrentVersion returns the latest version number, or 0 if no history.
func (r *Record) CurrentVersion() int {
	if len(r.History) == 0 {
		return 0
	}
	return r.History[len(r.History)-1].V
}

// CurrentFingerprint returns the latest fingerprint, or "" if no history.
func (r *Record) CurrentFingerprint() string {
	if len(r.History) == 0 {
		return ""
	}
	return r.History[len(r.History)-1].Fingerprint
}

// AppendVersion adds a new version record. Validates sequential numbering.
func (r *Record) AppendVersion(v Version) error {
	expected := r.CurrentVersion() + 1
	if v.V != expected {
		return fmt.Errorf("version number must be %d, got %d", expected, v.V)
	}
	if !fingerprintRe.MatchString(v.Fingerprint) {
		return fmt.Errorf("invalid fingerprint format: %s", v.Fingerprint)
	}
	r.History = append(r.History, v)
	return nil
}

// AppendEvent adds an event to the event log.
func (r *Record) AppendEvent(e Event) {
	r.Events = append(r.Events, e)
}

// StaleSignoffs returns all sign-offs where content has changed since approval.
func (r *Record) StaleSignoffs() []Signoff {
	current := r.CurrentFingerprint()
	var stale []Signoff
	for _, so := range r.Collaboration.Signoffs {
		if so.IsStale(current) {
			stale = append(stale, so)
		}
	}
	return stale
}

// ToJSON serializes the record to indented JSON.
func (r *Record) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FromJSON deserializes a record from JSON bytes.
func FromJSON(b []byte) (*Record, error) {
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("folio: invalid record JSON: %w", err)
	}
	return &r, nil
}

// ─── Utilities ────────────────────────────────────────────────────────────────

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// FormatFromExt maps file extensions to Folio format identifiers.
func FormatFromExt(ext string) string {
	switch ext {
	case ".docx":
		return "docx"
	case ".odt":
		return "odt"
	case ".ods":
		return "ods"
	case ".odp":
		return "odp"
	case ".pdf":
		return "pdf"
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "plain"
	case ".html", ".htm":
		return "html"
	case ".epub":
		return "epub"
	case ".rst":
		return "rst"
	case ".tex":
		return "latex"
	default:
		return "unknown"
	}
}
