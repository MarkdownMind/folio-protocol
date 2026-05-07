// Package fingerprint implements the FLP-0004 integrity model.
// Documents are fingerprinted via their Pandoc AST JSON representation.
// PDF documents use content-stream fingerprinting as a fallback.
//
// Dependencies: pandoc system binary (pandoc.org)
// No Go dependencies beyond stdlib.
package fingerprint

import (
"bytes"
"crypto/sha256"
"encoding/json"
"fmt"
"os"
"os/exec"
"path/filepath"
"sort"
"strings"
)

// noiseMetaKeys are metadata fields stripped before hashing.
// These change without semantic content change (FLP-0004 §3.2).
var noiseMetaKeys = map[string]bool{
"date":           true,
"generator":      true,
"producer":       true,
"modified":       true,
"revision":       true,
"editing-cycles": true,
"creation-date":  true,
"template":       true,
"last-printed":   true,
}

// Result holds the output of fingerprinting.
type Result struct {
Fingerprint string // "sha256:" + 64 hex chars
ASTVersion  string // Pandoc API version e.g. "3.2.1", or "pdf-streams-v1"
}

// Compute computes a Folio fingerprint for any supported document.
// For PDF files it uses content-stream hashing.
// For all other formats it uses the Pandoc AST pipeline.
func Compute(documentPath string) (Result, error) {
ext := strings.ToLower(filepath.Ext(documentPath))
if ext == ".pdf" {
return computePDF(documentPath)
}
return computeViaPandoc(documentPath)
}

// Verify checks whether a document's current content matches a stored fingerprint.
func Verify(documentPath, storedFingerprint, storedASTVersion string) (bool, string, error) {
result, err := Compute(documentPath)
if err != nil {
return false, "", err
}
return result.Fingerprint == storedFingerprint, result.Fingerprint, nil
}

// PandocVersion returns the installed Pandoc version string.
func PandocVersion() string {
out, err := exec.Command("pandoc", "--version").Output()
if err != nil {
return "not found"
}
lines := strings.SplitN(string(out), "\n", 2)
if len(lines) == 0 {
return "unknown"
}
parts := strings.Fields(lines[0])
if len(parts) < 2 {
return "unknown"
}
return parts[1]
}

// ─── Pandoc AST pipeline ─────────────────────────────────────────────────────

func computeViaPandoc(path string) (Result, error) {
if _, err := exec.LookPath("pandoc"); err != nil {
return Result{}, fmt.Errorf(
"pandoc not found: install from pandoc.org\n" +
"pandoc is required for document fingerprinting",
)
}

cmd := exec.Command("pandoc", path, "-t", "json")
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr

if err := cmd.Run(); err != nil {
return Result{}, fmt.Errorf("pandoc failed on %s: %s", path, stderr.String())
}

var ast map[string]interface{}
if err := json.Unmarshal(stdout.Bytes(), &ast); err != nil {
return Result{}, fmt.Errorf("pandoc AST parse error: %w", err)
}

apiVersion := extractAPIVersion(ast)
delete(ast, "pandoc-api-version")

stripMetaNoise(ast)
normalizeTextNodes(ast)

canonical, err := marshalCanonical(ast)
if err != nil {
return Result{}, fmt.Errorf("canonicalization error: %w", err)
}

hash := sha256.Sum256(canonical)
fp := fmt.Sprintf("sha256:%x", hash)

return Result{Fingerprint: fp, ASTVersion: apiVersion}, nil
}

func extractAPIVersion(ast map[string]interface{}) string {
raw, ok := ast["pandoc-api-version"]
if !ok {
return "unknown"
}
parts, ok := raw.([]interface{})
if !ok {
return "unknown"
}
strs := make([]string, len(parts))
for i, p := range parts {
strs[i] = fmt.Sprintf("%v", p)
}
return strings.Join(strs, ".")
}

func stripMetaNoise(ast map[string]interface{}) {
meta, ok := ast["meta"].(map[string]interface{})
if !ok {
return
}
for key := range noiseMetaKeys {
delete(meta, key)
}
}

func normalizeTextNodes(node interface{}) {
switch n := node.(type) {
case map[string]interface{}:
if t, ok := n["t"].(string); ok && t == "Str" {
if s, ok := n["c"].(string); ok {
n["c"] = strings.Join(strings.Fields(s), " ")
}
}
for _, v := range n {
normalizeTextNodes(v)
}
case []interface{}:
for _, item := range n {
normalizeTextNodes(item)
}
}
}

func marshalCanonical(v interface{}) ([]byte, error) {
return marshalSorted(v)
}

func marshalSorted(v interface{}) ([]byte, error) {
switch val := v.(type) {
case map[string]interface{}:
keys := make([]string, 0, len(val))
for k := range val {
keys = append(keys, k)
}
sort.Strings(keys)

var buf bytes.Buffer
buf.WriteByte('{')
for i, k := range keys {
if i > 0 {
buf.WriteByte(',')
}
keyJSON, _ := json.Marshal(k)
buf.Write(keyJSON)
buf.WriteByte(':')
valJSON, err := marshalSorted(val[k])
if err != nil {
return nil, err
}
buf.Write(valJSON)
}
buf.WriteByte('}')
return buf.Bytes(), nil

case []interface{}:
var buf bytes.Buffer
buf.WriteByte('[')
for i, item := range val {
if i > 0 {
buf.WriteByte(',')
}
itemJSON, err := marshalSorted(item)
if err != nil {
return nil, err
}
buf.Write(itemJSON)
}
buf.WriteByte(']')
return buf.Bytes(), nil

default:
return json.Marshal(v)
}
}

// ─── PDF fingerprinting ──────────────────────────────────────────────────────

func computePDF(path string) (Result, error) {
cmd := exec.Command("pandoc", path, "-t", "plain")
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr

if err := cmd.Run(); err != nil {
return computeRawFileHash(path, "pdf-raw-v1")
}

text := bytes.ReplaceAll(stdout.Bytes(), []byte("\r\n"), []byte("\n"))
text = bytes.TrimSpace(text)

hash := sha256.Sum256(text)
fp := fmt.Sprintf("sha256:%x", hash)

return Result{Fingerprint: fp, ASTVersion: "pdf-text-v1"}, nil
}

func computeRawFileHash(path, versionTag string) (Result, error) {
data, err := os.ReadFile(path)
if err != nil {
return Result{}, fmt.Errorf("cannot read file %s: %w", path, err)
}

hash := sha256.Sum256(data)
fp := fmt.Sprintf("sha256:%x", hash)
return Result{Fingerprint: fp, ASTVersion: versionTag}, nil
}
