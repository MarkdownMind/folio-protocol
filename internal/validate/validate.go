// Package validate implements the FLP-0005 conformance checker.
// A record passes validation when Record returns an empty slice.
package validate

import (
"fmt"
"regexp"
"time"

"github.com/MarkdownMind/folio-protocol/internal/core"
)

var (
fingerprintRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
folioUrnRe    = regexp.MustCompile(
`^urn:folio:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)
)

// ValidationError is a single conformance violation.
type ValidationError struct {
Field   string
Message string
}

func (e ValidationError) Error() string {
return fmt.Sprintf("[%s] %s", e.Field, e.Message)
}

// Record validates a Folio record against FLP-0001 and FLP-0005.
// Returns a slice of all violations found. Empty slice = valid.
func Record(r *core.Record) []ValidationError {
var errs []ValidationError

if r.Folio != core.ProtocolVersion {
errs = append(errs, ValidationError{
Field:   "folio",
Message: fmt.Sprintf("unsupported protocol version %q, expected %q", r.Folio, core.ProtocolVersion),
})
}

errs = append(errs, validateIdentity(r.Identity)...)
errs = append(errs, validateHistory(r.History)...)
errs = append(errs, validateCollaboration(r.Collaboration, r.History)...)

return errs
}

func validateIdentity(id core.Identity) []ValidationError {
var errs []ValidationError

if !folioUrnRe.MatchString(id.ID) {
errs = append(errs, ValidationError{
Field:   "identity.id",
Message: fmt.Sprintf("invalid Folio URN format: %q", id.ID),
})
}

if id.CreatedBy == "" {
errs = append(errs, ValidationError{
Field:   "identity.created-by",
Message: "required field is empty",
})
}

if err := validateTimestamp("identity.created", id.Created); err != nil {
errs = append(errs, *err)
}

for i, parent := range id.Lineage {
if !folioUrnRe.MatchString(parent) {
errs = append(errs, ValidationError{
Field:   fmt.Sprintf("identity.lineage[%d]", i),
Message: fmt.Sprintf("invalid Folio URN: %q", parent),
})
}
}

return errs
}

func validateHistory(history []core.Version) []ValidationError {
var errs []ValidationError

if len(history) == 0 {
return []ValidationError{{
Field:   "history",
Message: "must contain at least one version record",
}}
}

for i, v := range history {
prefix := fmt.Sprintf("history[%d]", i)
expected := i + 1

if v.V != expected {
errs = append(errs, ValidationError{
Field:   prefix + ".v",
Message: fmt.Sprintf("version gap: expected %d, got %d", expected, v.V),
})
}

if !fingerprintRe.MatchString(v.Fingerprint) {
errs = append(errs, ValidationError{
Field:   prefix + ".fingerprint",
Message: fmt.Sprintf("invalid format %q — must be sha256: + 64 hex chars", v.Fingerprint),
})
}

if err := validateTimestamp(prefix+".timestamp", v.Timestamp); err != nil {
errs = append(errs, *err)
}

if v.Author == "" {
errs = append(errs, ValidationError{
Field:   prefix + ".author",
Message: "required field is empty",
})
}

if v.ASTVersion == "" {
errs = append(errs, ValidationError{
Field:   prefix + ".ast-version",
Message: "required field is empty",
})
}
}

return errs
}

func validateCollaboration(c core.Collaboration, history []core.Version) []ValidationError {
var errs []ValidationError

validVersions := map[int]bool{}
validFingerprints := map[string]bool{}
for _, v := range history {
validVersions[v.V] = true
validFingerprints[v.Fingerprint] = true
}

markupIDs := map[string]bool{}
for i, m := range c.Markups {
prefix := fmt.Sprintf("collaboration.markups[%d]", i)

if m.ID == "" {
errs = append(errs, ValidationError{
Field: prefix + ".id", Message: "required field is empty",
})
} else if markupIDs[m.ID] {
errs = append(errs, ValidationError{
Field: prefix + ".id", Message: fmt.Sprintf("duplicate markup ID %q", m.ID),
})
}
markupIDs[m.ID] = true

if !validVersions[m.BaseVersion] {
errs = append(errs, ValidationError{
Field:   prefix + ".base-version",
Message: fmt.Sprintf("references non-existent version %d", m.BaseVersion),
})
}

if !fingerprintRe.MatchString(m.BaseFingerprint) {
errs = append(errs, ValidationError{
Field:   prefix + ".base-fingerprint",
Message: "invalid fingerprint format",
})
}

validStatuses := map[core.MarkupStatus]bool{
core.MarkupPending:      true,
core.MarkupIncorporated: true,
core.MarkupDeclined:     true,
core.MarkupExpired:      true,
}
if !validStatuses[m.Status] {
errs = append(errs, ValidationError{
Field:   prefix + ".status",
Message: fmt.Sprintf("invalid status %q", m.Status),
})
}
}

signoffIDs := map[string]bool{}
for i, so := range c.Signoffs {
prefix := fmt.Sprintf("collaboration.signoffs[%d]", i)

if so.ID == "" {
errs = append(errs, ValidationError{
Field: prefix + ".id", Message: "required field is empty",
})
} else if signoffIDs[so.ID] {
errs = append(errs, ValidationError{
Field: prefix + ".id", Message: fmt.Sprintf("duplicate signoff ID %q", so.ID),
})
}
signoffIDs[so.ID] = true

if !validVersions[so.Version] {
errs = append(errs, ValidationError{
Field:   prefix + ".version",
Message: fmt.Sprintf("references non-existent version %d", so.Version),
})
}

if !fingerprintRe.MatchString(so.FingerprintAtSignoff) {
errs = append(errs, ValidationError{
Field:   prefix + ".fingerprint-at-signoff",
Message: "invalid fingerprint format",
})
}
}

return errs
}

func validateTimestamp(field, ts string) *ValidationError {
if ts == "" {
return &ValidationError{Field: field, Message: "required field is empty"}
}
formats := []string{time.RFC3339, "2006-01-02T15:04:05Z"}
for _, f := range formats {
if _, err := time.Parse(f, ts); err == nil {
return nil
}
}
return &ValidationError{
Field:   field,
Message: fmt.Sprintf("invalid ISO8601 timestamp: %q", ts),
}
}
