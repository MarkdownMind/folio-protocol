# Folio Protocol — Design Rationale

This document records why specific design decisions were made. 
It is intended for implementers, contributors, and anyone evaluating 
whether to build on Folio.

---

## Why Pandoc AST and not a custom CDM?

The first instinct was to design a Canonical Document Model (CDM) — 
a purpose-built, format-agnostic semantic representation. After analysis, 
we concluded Pandoc's AST is that CDM.

Pandoc has been solving the format-agnostic document representation 
problem since 2006. Its AST handles Blocks (paragraphs, headers, tables, 
lists) and Inlines (text, emphasis, links) in a well-defined, documented 
JSON format. It supports 40+ formats. It is MIT licensed. It has 
implementations in Haskell, Python, Lua, R, and JavaScript.

Building a custom CDM would mean:
- Writing readers for every format (Pandoc already has them)
- Maintaining format compatibility as formats evolve (Pandoc does this)
- Convincing implementers to trust our AST (Pandoc is already trusted)

The only gap Pandoc doesn't fill: it has no concept of document identity, 
version history, collaboration, or cryptographic integrity. That is exactly 
what Folio adds.

---

## Why is Pandoc AST lossy for DOCX, and why is that acceptable?

Pandoc's own documentation states: "Pandoc attempts to preserve the 
structural elements of a document, but not formatting details such as 
margin size. Some document elements, such as complex tables, may not 
fit into pandoc's simple document model."

This lossiness is acceptable for Folio because:

1. Folio fingerprints and diffs **content meaning**, not visual appearance.
   A contract's legal meaning is in its text and structure, not its margins.

2. The original format file is always preserved intact. Folio never 
   modifies or replaces the source document. The DOCX or ODT or PDF 
   is unchanged. Folio's record rides alongside it.

3. For legal and professional documents, content changes are what matter
   for audit and version control purposes. Formatting changes are surfaced 
   separately as `format` operation type in FLP-0002.

4. The alternative — fingerprinting raw OOXML bytes — would produce 
   different fingerprints for the same content saved by different versions 
   of Word, or on different machines, due to rsid pollution and other noise.
   The Pandoc AST approach is actually MORE stable for content fingerprinting 
   than the raw file bytes.

---

## Why JSON and not XML?

The Folio record is JSON. Not XML.

The transport mechanisms (DOCX Custom XML Parts, ODF META-INF/) are 
XML-based containers, but the Folio record content is JSON.

Reasons:
- JSON is simpler to parse in every language
- JSON diffs more cleanly in version control
- JSON is what developers expect for structured data in 2026
- The record structure (arrays, nested objects) maps naturally to JSON
- XML would add schema namespace complexity with no benefit

The DOCX Custom XML Parts spec allows any content type, not just XML. 
The ODF META-INF/ directory accepts any file. The XMP spec supports 
custom namespace properties that can carry any string value. 
None of these require the payload to be XML.

---

## Why not build on git directly?

Git is the obvious prior art. It solves version control, identity, 
diffing, and merging for code. Why not just use git for documents?

Four reasons:

1. **Binary formats.** Git diffs binary files as opaque blobs. A DOCX 
   is a binary (ZIP) file. git diff on two DOCX files produces nothing 
   useful. Folio's Pandoc AST layer provides the semantic diff git cannot.

2. **No embedded history.** Git history lives in the `.git/` repository. 
   Send the file to a counterparty and the history stays behind. Folio 
   history travels with the file.

3. **UX.** Lawyers and engineers are not going to use a terminal. 
   Folio's vocabulary (track, save, redline, sign-off, milestone) maps 
   to how document professionals already think. Git's vocabulary (commit, 
   push, rebase, HEAD) does not.

4. **Multi-party without a shared server.** Git requires a shared remote 
   (GitHub, GitLab, etc.) for collaboration. Folio's markup/signoff model 
   works through email — send the file, get it back with proposed changes 
   embedded, incorporate them. No shared infrastructure needed.

Folio borrows git's concepts (identity, append-only history, semantic 
diff, merge with conflict detection) while solving git's document-specific 
limitations.

---

## Why the pen-holder model?

Legal document collaboration is not symmetric. One person — the 
pen-holder — owns the document and is responsible for its coherence. 
Specialists provide bounded input within their domain. The pen-holder 
decides what to incorporate.

This is not a limitation to work around. It is a feature. The pen-holder 
model exists because legal documents can create binding obligations worth 
millions of dollars. A specialist who blindly accepts changes from another 
specialist abdicates their control responsibility.

Folio's markup/signoff model respects this reality:
- Contributors propose (markup), they do not commit
- Only the pen-holder incorporates markups
- Sign-offs are anchored to specific version fingerprints
- Stale sign-off detection alerts when content changes after approval

Any system that gives all collaborators equal commit authority will 
fail in legal and regulated document workflows.

---

## Why local-first?

Attorney-client privilege is a legal doctrine, not a preference. 
A lawyer who routes client documents through a third-party cloud 
service without informed consent may be violating their ethical 
obligations. The same applies to doctor-patient confidentiality, 
trade secret protection, and many regulated industries.

Cloud-based document version control tools require trust in a third 
party. For most professional document use cases, that trust cannot be 
assumed.

Local-first means:
- The Folio record is embedded in the file itself
- Reading the record requires no network connection
- No third-party server ever sees the document content
- The user controls their data entirely

This is also a competitive advantage. "Your documents never leave your 
network" is a sale in regulated industries. It is not a sale if it 
requires a custom server deployment — which is why the sidecar model 
and embedded transports are so important. They work with zero 
infrastructure.

---

## Why MIT and not a more restrictive license?

A protocol nobody can implement without paying a license fee is not 
a protocol — it is a product. 

Protocols succeed through adoption. Adoption requires zero friction. 
MIT means:
- Anyone can implement it
- Anyone can build on it commercially
- Anyone can fork it
- No legal review required to use it

The business model for Folio is not the spec. It is the tooling, 
the add-ins, the hardware appliance, and the vertical applications 
built on top of the open protocol. The spec itself is a public good.

This is the same model that made HTTP, SMTP, JSON, and Markdown 
successful. Nobody owns them. Everyone builds on them.

---

## What we deliberately did not build

**A new document format.** DOCX, ODF, and PDF are permanent. 
Building a new format that nobody opens is a whitepaper.

**A cloud platform.** Simul Docs and Version Story already exist. 
The gap is local-first + protocol, not another cloud app.

**A DRM system.** Folio records events. It does not restrict access. 
Access control is a separate concern handled by the host system.

**A legal signing standard.** Folio records sign-off events with 
content-anchored fingerprints. It does not replace DocuSign or 
qualified electronic signatures. FLP-0016 will address optional 
cryptographic signing for use cases that need it.

**A replacement for Word or LibreOffice.** Users keep their tools. 
The protocol works with any application that can read and write 
the host format.
