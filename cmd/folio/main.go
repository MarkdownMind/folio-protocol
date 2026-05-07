// folio — Universal document version tracking
//
// Build:
//   GOOS=windows GOARCH=amd64 go build -o folio.exe ./cmd/folio
//   GOOS=darwin  GOARCH=arm64 go build -o folio     ./cmd/folio
//   GOOS=linux   GOARCH=amd64 go build -o folio     ./cmd/folio
//
// Zero runtime dependencies. Pandoc (pandoc.org) must be installed separately.
package main

import (
"bytes"
"fmt"
"os"
"os/exec"
"path/filepath"
"strconv"
"strings"
"time"

"github.com/MarkdownMind/folio-protocol/internal/core"
"github.com/MarkdownMind/folio-protocol/internal/fingerprint"
"github.com/MarkdownMind/folio-protocol/internal/transport"
"github.com/MarkdownMind/folio-protocol/internal/validate"
)

func main() {
if len(os.Args) < 2 {
printUsage()
os.Exit(0)
}

cmd := strings.ToLower(os.Args[1])
args := os.Args[2:]

var exitCode int
switch cmd {
case "track":
exitCode = cmdTrack(args)
case "save":
exitCode = cmdSave(args)
case "history":
exitCode = cmdHistory(args)
case "redline":
exitCode = cmdRedline(args)
case "verify":
exitCode = cmdVerify(args)
case "convert":
exitCode = cmdConvert(args)
case "milestone":
exitCode = cmdMilestone(args)
case "status":
exitCode = cmdStatus(args)
case "validate":
exitCode = cmdValidate(args)
case "version":
exitCode = cmdVersion()
default:
fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
printUsage()
exitCode = 1
}

os.Exit(exitCode)
}

// ─── Commands ─────────────────────────────────────────────────────────────────

func cmdTrack(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
author := flagString(args, "--author", defaultAuthor())
title := flagString(args, "--title", "")
note := flagString(args, "--note", "Initial version")

if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)

existing, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if existing != nil {
fmt.Printf("✗ Already tracking: %s\n", filepath.Base(path))
fmt.Printf("  Document ID: %s\n", existing.Identity.ID)
fmt.Printf("  Use 'folio save' to record changes.\n")
return 1
}

fp, err := fingerprint.Compute(path)
if err != nil {
return fail("fingerprint error: %v", err)
}

identity := core.NewIdentity(author, title)
record := core.NewRecord(identity, author)

v := core.Version{
V:           1,
Author:      author,
Timestamp:   utcNow(),
Fingerprint: fp.Fingerprint,
ASTVersion:  fp.ASTVersion,
Note:        note,
Format:      core.FormatFromExt(filepath.Ext(path)),
}
if err := record.AppendVersion(v); err != nil {
return fail("version error: %v", err)
}

record.AppendEvent(core.Event{
Event:     core.EventInitialized,
Timestamp: utcNow(),
By:        author,
Data:      map[string]interface{}{"format": core.FormatFromExt(filepath.Ext(path))},
})

if errs := validate.Record(record); len(errs) > 0 {
fmt.Fprintln(os.Stderr, "✗ Validation errors:")
for _, e := range errs {
fmt.Fprintf(os.Stderr, "  %s\n", e)
}
return 1
}

if err := t.Write(path, record); err != nil {
return fail("write error: %v", err)
}

fmt.Printf("✓ Started tracking: %s\n", filepath.Base(path))
fmt.Printf("  Document ID:  %s\n", record.Identity.ID)
fmt.Printf("  Version:      1\n")
fmt.Printf("  Author:       %s\n", author)
fmt.Printf("  Format:       %s\n", core.FormatFromExt(filepath.Ext(path)))
if title != "" {
fmt.Printf("  Title:        %s\n", title)
}
return 0
}

func cmdSave(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
author := flagString(args, "--author", defaultAuthor())
note := flagString(args, "--note", "")

if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)
record, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
fmt.Fprintf(os.Stderr, "✗ Not tracked. Run 'folio track %s' first.\n",
filepath.Base(path))
return 1
}

fp, err := fingerprint.Compute(path)
if err != nil {
return fail("fingerprint error: %v", err)
}

if fp.Fingerprint == record.CurrentFingerprint() {
fmt.Printf("  No changes since version %d. Nothing saved.\n",
record.CurrentVersion())
return 0
}

nextV := record.CurrentVersion() + 1
v := core.Version{
V:           nextV,
Author:      author,
Timestamp:   utcNow(),
Fingerprint: fp.Fingerprint,
ASTVersion:  fp.ASTVersion,
Note:        note,
Format:      core.FormatFromExt(filepath.Ext(path)),
}
if err := record.AppendVersion(v); err != nil {
return fail("version error: %v", err)
}

if err := t.Write(path, record); err != nil {
return fail("write error: %v", err)
}

fmt.Printf("✓ Saved version %d\n", nextV)
fmt.Printf("  Author: %s\n", author)
if note != "" {
fmt.Printf("  Note:   %s\n", note)
}
fmt.Printf("  Fingerprint: %s...\n", fp.Fingerprint[:20])
return 0
}

func cmdHistory(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)
record, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
fmt.Fprintf(os.Stderr, "✗ Not tracked: %s\n", filepath.Base(path))
return 1
}

title := record.Identity.Title
if title == "" {
title = filepath.Base(path)
}

fmt.Printf("\n%s\n", title)
fmt.Printf("ID: %s\n", record.Identity.ID)
fmt.Println(strings.Repeat("─", 60))

for i := len(record.History) - 1; i >= 0; i-- {
v := record.History[i]
ts := formatTimestamp(v.Timestamp)
fmt.Printf("  v%-4d %-30s %s\n", v.V, v.Author, ts)
if v.Note != "" {
fmt.Printf("       %s\n", v.Note)
}
}

var milestones []core.Event
for _, e := range record.Events {
if e.Event == core.EventMilestone {
milestones = append(milestones, e)
}
}
if len(milestones) > 0 {
fmt.Println("\nMilestones:")
for _, m := range milestones {
label, _ := m.Data["label"].(string)
ver, _ := m.Data["version"].(float64)
fmt.Printf("  [%s] at version %v\n", label, int(ver))
}
}

signoffs := record.Collaboration.Signoffs
if len(signoffs) > 0 {
fmt.Println("\nSign-offs:")
currentFP := record.CurrentFingerprint()
for _, so := range signoffs {
stale := so.IsStale(currentFP)
flag := " ✓"
if stale {
flag = " ⚠ STALE"
}
fmt.Printf("  %s signed off v%d%s\n", so.By, so.Version, flag)
}
}

fmt.Println()
return 0
}

func cmdVerify(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
targetV := flagInt(args, "--version", 0)

if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)
record, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
fmt.Fprintf(os.Stderr, "✗ Not tracked: %s\n", filepath.Base(path))
return 1
}

if targetV == 0 {
targetV = record.CurrentVersion()
}

var stored *core.Version
for _, v := range record.History {
if v.V == targetV {
vCopy := v
stored = &vCopy
break
}
}
if stored == nil {
return fail("version %d not found in history", targetV)
}

matches, currentFP, err := fingerprint.Verify(path, stored.Fingerprint, stored.ASTVersion)
if err != nil {
return fail("fingerprint error: %v", err)
}

if matches {
fmt.Printf("✓ Fingerprint verified — matches version %d\n", targetV)
fmt.Printf("  Content has not changed since v%d was recorded.\n", targetV)
fmt.Printf("  Fingerprint: %s...\n", stored.Fingerprint[:32])
return 0
}

fmt.Printf("⚠ Fingerprint mismatch\n")
fmt.Printf("  Current:  %s...\n", currentFP[:32])
fmt.Printf("  Stored v%d: %s...\n", targetV, stored.Fingerprint[:32])
if targetV == record.CurrentVersion() {
fmt.Printf("  Document has unsaved changes not yet tracked.\n")
} else {
fmt.Printf("  Document has been modified since v%d.\n", targetV)
}
return 2
}

func cmdStatus(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)
record, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
fmt.Printf("  %s: not tracked\n", filepath.Base(path))
return 0
}

fp, err := fingerprint.Compute(path)
if err != nil {
return fail("fingerprint error: %v", err)
}

status := "clean"
if fp.Fingerprint != record.CurrentFingerprint() {
status = "modified"
}

fmt.Printf("  %s\n", filepath.Base(path))
fmt.Printf("  Version: %d  Status: %s\n", record.CurrentVersion(), status)
fmt.Printf("  ID: %s\n", record.Identity.ID)
if status == "modified" {
fmt.Printf("  ⚠ Unsaved changes detected. Run 'folio save' to record.\n")
}

stale := record.StaleSignoffs()
if len(stale) > 0 {
fmt.Printf("  ⚠ %d stale sign-off(s) — content changed since approval.\n",
len(stale))
}
return 0
}

func cmdConvert(args []string) int {
if len(args) < 2 {
return fail("usage: folio convert <input> <output>")
}
src, dst := args[0], args[1]
author := flagString(args, "--author", defaultAuthor())

if err := requireExists(src); err != nil {
return fail(err.Error())
}

srcT := transport.For(src)
record, err := srcT.Read(src)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
fmt.Fprintf(os.Stderr, "✗ Source not tracked. Run 'folio track %s' first.\n",
filepath.Base(src))
return 1
}

out, err := runPandoc(src, dst)
if err != nil {
return fail("pandoc conversion failed: %s", out)
}

fp, err := fingerprint.Compute(dst)
if err != nil {
return fail("fingerprint error on output: %v", err)
}

srcFmt := core.FormatFromExt(filepath.Ext(src))
dstFmt := core.FormatFromExt(filepath.Ext(dst))

record.AppendEvent(core.Event{
Event:     core.EventConverted,
Timestamp: utcNow(),
By:        author,
Data: map[string]interface{}{
"from":              srcFmt,
"to":                dstFmt,
"version":           record.CurrentVersion(),
"fingerprint-after": fp.Fingerprint,
"ast-version":       fp.ASTVersion,
},
})

dstT := transport.For(dst)
if err := dstT.Write(dst, record); err != nil {
return fail("write error: %v", err)
}

fmt.Printf("✓ Converted: %s → %s\n", filepath.Base(src), filepath.Base(dst))
fmt.Printf("  Document ID preserved: %s\n", record.Identity.ID)
fmt.Printf("  New fingerprint: %s...\n", fp.Fingerprint[:32])
return 0
}

func cmdMilestone(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
label := flagString(args, "--label", "")
if label == "" {
return fail("--label is required")
}
author := flagString(args, "--author", defaultAuthor())

if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)
record, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
return fail("not tracked: %s", filepath.Base(path))
}

record.AppendEvent(core.Event{
Event:     core.EventMilestone,
Timestamp: utcNow(),
By:        author,
Data: map[string]interface{}{
"label":   label,
"version": record.CurrentVersion(),
},
})

if err := t.Write(path, record); err != nil {
return fail("write error: %v", err)
}

fmt.Printf("✓ Milestone '%s' marked at version %d\n", label, record.CurrentVersion())
return 0
}

func cmdValidate(args []string) int {
if len(args) == 0 {
return fail("document path required")
}
path := args[0]
if err := requireExists(path); err != nil {
return fail(err.Error())
}

t := transport.For(path)
record, err := t.Read(path)
if err != nil {
return fail("read error: %v", err)
}
if record == nil {
fmt.Printf("✗ No Folio record found in %s\n", filepath.Base(path))
return 1
}

errs := validate.Record(record)
if len(errs) == 0 {
fmt.Printf("✓ Valid Folio record (FLP-0001 / FLP-0005)\n")
fmt.Printf("  Document ID: %s\n", record.Identity.ID)
fmt.Printf("  Versions:    %d\n", len(record.History))
return 0
}

fmt.Printf("✗ %d conformance violation(s):\n", len(errs))
for _, e := range errs {
fmt.Printf("  %s\n", e)
}
return 1
}

func cmdRedline(args []string) int {
if len(args) < 2 {
return fail("usage: folio redline <doc1> <doc2>")
}
path1, path2 := args[0], args[1]
if err := requireExists(path1); err != nil {
return fail(err.Error())
}
if err := requireExists(path2); err != nil {
return fail(err.Error())
}

fp1, err := fingerprint.Compute(path1)
if err != nil {
return fail("fingerprint error on %s: %v", path1, err)
}
fp2, err := fingerprint.Compute(path2)
if err != nil {
return fail("fingerprint error on %s: %v", path2, err)
}

fmt.Printf("Redline: %s → %s\n", filepath.Base(path1), filepath.Base(path2))
fmt.Println(strings.Repeat("─", 60))

if fp1.Fingerprint == fp2.Fingerprint {
fmt.Println("  Documents have identical content fingerprints.")
fmt.Println("  No semantic differences detected.")
return 0
}

text1, err := pandocToPlain(path1)
if err != nil {
return fail("conversion error: %v", err)
}
text2, err := pandocToPlain(path2)
if err != nil {
return fail("conversion error: %v", err)
}

printLineDiff(text1, text2)
return 0
}

func cmdVersion() int {
fmt.Printf("folio %s\n", "1.0.0-alpha")
fmt.Printf("Pandoc: %s\n", fingerprint.PandocVersion())
fmt.Printf("Protocol: https://github.com/MarkdownMind/folio-protocol\n")
fmt.Printf("License: MIT\n")
fmt.Printf("\nBuild targets:\n")
fmt.Printf("  Windows: GOOS=windows GOARCH=amd64 go build -o folio.exe ./cmd/folio\n")
fmt.Printf("  macOS:   GOOS=darwin  GOARCH=arm64 go build -o folio     ./cmd/folio\n")
fmt.Printf("  Linux:   GOOS=linux   GOARCH=amd64 go build -o folio     ./cmd/folio\n")
return 0
}

// ─── Simple line diff ─────────────────────────────────────────────────────────

func printLineDiff(text1, text2 string) {
lines1 := strings.Split(text1, "\n")
lines2 := strings.Split(text2, "\n")

added, removed := 0, 0
maxLen := len(lines1)
if len(lines2) > maxLen {
maxLen = len(lines2)
}

fmt.Println()
for i := 0; i < maxLen; i++ {
l1 := ""
if i < len(lines1) {
l1 = lines1[i]
}
l2 := ""
if i < len(lines2) {
l2 = lines2[i]
}

if l1 == l2 {
continue
}
if l1 != "" && l2 == "" {
fmt.Printf("  - %s\n", l1)
removed++
} else if l1 == "" && l2 != "" {
fmt.Printf("  + %s\n", l2)
added++
} else {
fmt.Printf("  - %s\n", l1)
fmt.Printf("  + %s\n", l2)
added++
removed++
}
}

fmt.Printf("\n  %d addition(s), %d removal(s)\n", added, removed)
}

// ─── Utilities ────────────────────────────────────────────────────────────────

func runPandoc(src, dst string) (string, error) {
cmd := exec.Command("pandoc", src, "-o", dst)
var stderr bytes.Buffer
cmd.Stderr = &stderr
err := cmd.Run()
return stderr.String(), err
}

func pandocToPlain(path string) (string, error) {
cmd := exec.Command("pandoc", path, "-t", "plain")
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr
if err := cmd.Run(); err != nil {
return "", fmt.Errorf("pandoc: %s", stderr.String())
}
return stdout.String(), nil
}

func requireExists(path string) error {
if _, err := os.Stat(path); os.IsNotExist(err) {
return fmt.Errorf("file not found: %s", path)
}
return nil
}

func defaultAuthor() string {
if v := os.Getenv("FOLIO_AUTHOR"); v != "" {
return v
}
if v := os.Getenv("GIT_AUTHOR_NAME"); v != "" {
return v
}
if v := os.Getenv("USERNAME"); v != "" {
return v
}
if v := os.Getenv("USER"); v != "" {
return v
}
return "unknown"
}

func flagString(args []string, flag, defaultVal string) string {
for i, a := range args {
if a == flag && i+1 < len(args) {
return args[i+1]
}
}
return defaultVal
}

func flagInt(args []string, flag string, defaultVal int) int {
s := flagString(args, flag, "")
if s == "" {
return defaultVal
}
n, err := strconv.Atoi(s)
if err != nil {
return defaultVal
}
return n
}

func fail(format string, args ...interface{}) int {
fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
return 1
}

func utcNow() string {
return time.Now().UTC().Format(time.RFC3339)
}

func formatTimestamp(ts string) string {
t, err := time.Parse(time.RFC3339, ts)
if err != nil {
return ts
}
return t.Local().Format("Jan 2, 2006  3:04 PM")
}

func printUsage() {
fmt.Print(`
folio — Universal document version tracking
Single binary. No runtime. Any format.

USAGE
  folio <command> [options]

COMMANDS
  track    <doc>              Start tracking a document
  save     <doc>              Record a new version
  history  <doc>              Show version history
  redline  <doc1> <doc2>      Compare two documents
  verify   <doc>              Verify document fingerprint
  convert  <input> <output>   Convert format, preserve record
  milestone <doc>             Mark a significant version
  status   <doc>              Quick status check
  validate <doc>              Check record conformance
  version                     Show version information

OPTIONS
  --author <name>    Author identifier (or set FOLIO_AUTHOR env var)
  --title  <text>    Document title (track only)
  --note   <text>    Version description
  --label  <text>    Milestone label (milestone only)
  --version <n>      Specific version (verify only)

SUPPORTED FORMATS
  .docx  .odt  .ods  .odp  .pdf  .md  .txt  .html  .epub  .rst  .tex
  Any other format uses a .folio sidecar file automatically.

EXAMPLES
  folio track contract.docx --author ian@firm.com --title "NDA — Acme"
  folio track brief.pdf     --author ian@firm.com
  folio track notes.md      --author ian@firm.com
  folio save  contract.docx --note "Incorporated tax markup"
  folio history contract.docx
  folio redline contract_v1.docx contract_v2.odt
  folio verify  contract.docx
  folio convert contract.docx contract_final.pdf --author ian@firm.com
  folio milestone contract_final.pdf --label "Executed"

ENVIRONMENT
  FOLIO_AUTHOR   Default author for all commands

REQUIREMENTS
  pandoc must be installed: pandoc.org
  pandoc is a free, single-binary download.

`)
}
