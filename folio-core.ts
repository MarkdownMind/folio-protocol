/**
 * folio-js — Folio Protocol for Word add-ins
 * TypeScript / Office.js
 * 
 * Distributed via npm: @folioprotocol/office-js
 * Used inside a Word task pane add-in.
 * 
 * Transport: DOCX Custom XML Parts (FLP-0010)
 * Degrades to read-only on Word Online (FLP-0005 §2.1)
 */

// ─── Types (matching FLP-0001 JSON schema) ────────────────────────────────────

export interface FolioIdentity {
  id: string;           // urn:folio:{uuid-v4}
  title?: string;
  created: string;      // ISO8601 UTC
  "created-by": string;
  lineage?: string[];
}

export interface FolioVersion {
  v: number;
  author: string;
  timestamp: string;
  fingerprint: string;  // sha256:{64 hex}
  "ast-version": string;
  note?: string;
  format?: string;
}

export interface FolioOp {
  type: "modify" | "insert" | "delete" | "format" | "move";
  loc?: string;
  old?: string;
  new?: string;
  from?: string;
  to?: string;
  description?: string;
}

export interface FolioMarkup {
  id: string;
  from: string;
  "from-display"?: string;
  submitted: string;
  "base-version": number;
  "base-fingerprint": string;
  status: "pending" | "incorporated" | "declined" | "expired";
  "incorporated-in"?: number;
  note?: string;
  "decline-reason"?: string;
  ops?: FolioOp[];
}

export interface FolioSignoff {
  id: string;
  by: string;
  "by-display"?: string;
  timestamp: string;
  version: number;
  "fingerprint-at-signoff": string;
  scope?: string[];
}

export interface FolioCollaboration {
  "pen-holder": string | null;
  markups: FolioMarkup[];
  signoffs: FolioSignoff[];
}

export interface FolioRecord {
  folio: "1.0";
  identity: FolioIdentity;
  history: FolioVersion[];
  events: FolioEvent[];
  collaboration: FolioCollaboration;
}

export interface FolioEvent {
  event: string;
  timestamp: string;
  by: string;
  [key: string]: unknown;
}

// ─── Platform detection ───────────────────────────────────────────────────────

/**
 * Returns true if running in Word Online (browser).
 * Word Online has known CustomXmlParts write instability (FLP-0005 §2.1).
 * On Word Online, the add-in operates in read-only mode.
 */
export function isWordOnline(): boolean {
  return Office.context.platform === Office.PlatformType.OfficeOnline;
}

export function canWrite(): boolean {
  return !isWordOnline();
}

// ─── Custom XML namespace ─────────────────────────────────────────────────────

const FOLIO_NAMESPACE = "http://schemas.folioprotocol.io/v1";

// Maximum part size per FLP-0005 §2.2 (Word Online 1MB hard limit)
const MAX_PART_SIZE_BYTES = 900 * 1024; // 900KB

// ─── Read ─────────────────────────────────────────────────────────────────────

/**
 * Reads the Folio record from the current Word document.
 * Returns null if the document has no Folio record.
 * 
 * @throws if the Office API call fails
 */
export async function readRecord(): Promise<FolioRecord | null> {
  return new Promise((resolve, reject) => {
    Office.context.document.customXmlParts.getByNamespaceAsync(
      FOLIO_NAMESPACE,
      (result) => {
        if (result.status !== Office.AsyncResultStatus.Succeeded) {
          reject(new Error(`CustomXmlParts read failed: ${result.error.message}`));
          return;
        }

        const parts = result.value;
        if (!parts || parts.length === 0) {
          resolve(null); // no Folio record
          return;
        }

        parts[0].getXmlAsync((xmlResult) => {
          if (xmlResult.status !== Office.AsyncResultStatus.Succeeded) {
            reject(new Error(`XML read failed: ${xmlResult.error.message}`));
            return;
          }

          try {
            // The record is stored as JSON inside the XML wrapper
            const record = parseRecordFromXmlPart(xmlResult.value);
            resolve(record);
          } catch (err) {
            reject(new Error(`Folio record parse error: ${(err as Error).message}`));
          }
        });
      }
    );
  });
}

// ─── Write ────────────────────────────────────────────────────────────────────

/**
 * Writes a Folio record to the current Word document.
 * 
 * Degrades gracefully on Word Online (returns false, does not throw).
 * Size is checked against the 900KB limit before writing.
 * 
 * @returns true if written, false if on Word Online (read-only degradation)
 */
export async function writeRecord(record: FolioRecord): Promise<boolean> {
  if (!canWrite()) {
    console.warn(
      "Folio: Word Online detected. Version tracking requires Word Desktop. " +
      "History is visible but cannot be updated here. (FLP-0005 §2.1)"
    );
    return false;
  }

  const xml = buildXmlPart(record);

  // Size check (FLP-0005 §2.2)
  const sizeBytes = new TextEncoder().encode(xml).length;
  if (sizeBytes > MAX_PART_SIZE_BYTES) {
    throw new Error(
      `Folio record size ${sizeBytes} bytes exceeds 900KB Word Online limit. ` +
      "Compaction required before saving."
    );
  }

  // Remove existing Folio parts
  await deleteExistingParts();

  // Write new part
  return new Promise((resolve, reject) => {
    Office.context.document.customXmlParts.addAsync(
      xml,
      (result) => {
        if (result.status === Office.AsyncResultStatus.Succeeded) {
          resolve(true);
        } else {
          reject(new Error(`CustomXmlParts write failed: ${result.error.message}`));
        }
      }
    );
  });
}

// ─── Initialize ───────────────────────────────────────────────────────────────

/**
 * Initializes Folio tracking on a document that has no record.
 * Creates version 1 with a fingerprint of the current content.
 */
export async function initializeRecord(
  author: string,
  title?: string,
  note: string = "Initial version"
): Promise<FolioRecord> {
  const existing = await readRecord();
  if (existing) {
    throw new Error("Document already has a Folio record.");
  }

  const id = generateFolioUrn();
  const now = utcNow();
  const fingerprint = await computeFingerprint();

  const record: FolioRecord = {
    folio: "1.0",
    identity: {
      id,
      title: title || "",
      created: now,
      "created-by": author,
    },
    history: [
      {
        v: 1,
        author,
        timestamp: now,
        fingerprint,
        "ast-version": "office-js-v1",
        note,
        format: "docx",
      },
    ],
    events: [
      {
        event: "INITIALIZED",
        timestamp: now,
        by: author,
        format: "docx",
      },
    ],
    collaboration: {
      "pen-holder": author,
      markups: [],
      signoffs: [],
    },
  };

  await writeRecord(record);
  return record;
}

// ─── Save version ─────────────────────────────────────────────────────────────

/**
 * Records a new version if document content has changed.
 * Hook this into the document's save event.
 * 
 * Returns the new version number, or null if no change detected.
 */
export async function saveVersion(
  author: string,
  note?: string
): Promise<number | null> {
  const record = await readRecord();
  if (!record) return null;

  const currentFingerprint = await computeFingerprint();
  const lastFingerprint = record.history[record.history.length - 1]?.fingerprint;

  if (currentFingerprint === lastFingerprint) {
    return null; // no change
  }

  const nextV = record.history[record.history.length - 1].v + 1;
  const now = utcNow();

  record.history.push({
    v: nextV,
    author,
    timestamp: now,
    fingerprint: currentFingerprint,
    "ast-version": "office-js-v1",
    note,
    format: "docx",
  });

  await writeRecord(record);
  return nextV;
}

// ─── Stale sign-off detection ─────────────────────────────────────────────────

/**
 * Returns sign-offs where content has changed since approval.
 * Used to surface warnings in the sidebar.
 */
export function getStaleSignoffs(record: FolioRecord): FolioSignoff[] {
  const currentFP = record.history[record.history.length - 1]?.fingerprint;
  if (!currentFP) return [];

  return record.collaboration.signoffs.filter(
    (so) => so["fingerprint-at-signoff"] !== currentFP
  );
}

// ─── Fingerprinting ───────────────────────────────────────────────────────────

/**
 * Computes a fingerprint of the current document content.
 * 
 * Full FLP-0004 fingerprinting (Pandoc AST + C14N + SHA-256) requires
 * the server-side component or folio.exe. The Office.js add-in uses
 * a SHA-256 of the raw OOXML bytes as an approximation, flagged with
 * ast-version "office-js-v1" to indicate the method.
 * 
 * For production accuracy, the add-in should delegate fingerprinting
 * to the local Folio server node (folio-go) via a local HTTP endpoint.
 */
export async function computeFingerprint(): Promise<string> {
  return new Promise((resolve, reject) => {
    Office.context.document.getFileAsync(
      Office.FileType.Compressed,
      { sliceSize: 65536 },
      async (result) => {
        if (result.status !== Office.AsyncResultStatus.Succeeded) {
          // Fallback: hash the document text
          resolve(await computeTextFingerprint());
          return;
        }

        const file = result.value;
        const slices: ArrayBuffer[] = [];
        let sliceIndex = 0;

        const getNextSlice = () => {
          file.getSliceAsync(sliceIndex, (sliceResult) => {
            if (sliceResult.status !== Office.AsyncResultStatus.Succeeded) {
              file.closeAsync();
              computeFingerprintFromBytes(concatBuffers(slices)).then(resolve);
              return;
            }
            slices.push(sliceResult.value.data);
            sliceIndex++;
            if (sliceIndex < file.sliceCount) {
              getNextSlice();
            } else {
              file.closeAsync();
              computeFingerprintFromBytes(concatBuffers(slices)).then(resolve);
            }
          });
        };
        getNextSlice();
      }
    );
  });
}

async function computeTextFingerprint(): Promise<string> {
  return new Promise((resolve) => {
    Office.context.document.getSelectedDataAsync(
      Office.CoercionType.Text,
      (result) => {
        hashString(result.value || "").then(resolve);
      }
    );
  });
}

async function computeFingerprintFromBytes(buffer: ArrayBuffer): Promise<string> {
  const hashBuffer = await crypto.subtle.digest("SHA-256", buffer);
  const hex = Array.from(new Uint8Array(hashBuffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  return `sha256:${hex}`;
}

async function hashString(s: string): Promise<string> {
  return computeFingerprintFromBytes(new TextEncoder().encode(s).buffer);
}

function concatBuffers(buffers: ArrayBuffer[]): ArrayBuffer {
  const total = buffers.reduce((sum, b) => sum + b.byteLength, 0);
  const result = new Uint8Array(total);
  let offset = 0;
  for (const buf of buffers) {
    result.set(new Uint8Array(buf), offset);
    offset += buf.byteLength;
  }
  return result.buffer;
}

// ─── XML Part serialization ───────────────────────────────────────────────────

/**
 * Wraps the Folio JSON record in a minimal XML document for the
 * Custom XML Parts API. The JSON is embedded as CDATA.
 */
function buildXmlPart(record: FolioRecord): string {
  const json = JSON.stringify(record, null, 2);
  return (
    `<?xml version="1.0" encoding="UTF-8"?>\n` +
    `<folio xmlns="${FOLIO_NAMESPACE}"><![CDATA[${json}]]></folio>`
  );
}

/**
 * Extracts the Folio JSON record from a Custom XML Part string.
 * Handles both CDATA-wrapped JSON and direct JSON content.
 */
function parseRecordFromXmlPart(xml: string): FolioRecord {
  // Extract CDATA content
  const cdataMatch = xml.match(/<!\[CDATA\[([\s\S]*?)\]\]>/);
  if (cdataMatch) {
    return JSON.parse(cdataMatch[1]) as FolioRecord;
  }

  // Try direct JSON between tags
  const tagMatch = xml.match(/<folio[^>]*>([\s\S]*?)<\/folio>/);
  if (tagMatch) {
    return JSON.parse(tagMatch[1].trim()) as FolioRecord;
  }

  throw new Error("Cannot find Folio record in XML part");
}

// ─── Utilities ────────────────────────────────────────────────────────────────

function generateFolioUrn(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant
  const hex = Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  const uuid = [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20),
  ].join("-");
  return `urn:folio:${uuid}`;
}

function utcNow(): string {
  return new Date().toISOString().replace(/\.\d{3}Z$/, "Z");
}

async function deleteExistingParts(): Promise<void> {
  return new Promise((resolve) => {
    Office.context.document.customXmlParts.getByNamespaceAsync(
      FOLIO_NAMESPACE,
      (result) => {
        if (
          result.status !== Office.AsyncResultStatus.Succeeded ||
          !result.value?.length
        ) {
          resolve();
          return;
        }
        let remaining = result.value.length;
        for (const part of result.value) {
          part.deleteAsync(() => {
            remaining--;
            if (remaining === 0) resolve();
          });
        }
      }
    );
  });
}
