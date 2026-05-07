// FolioDocument.cs
// folio-dotnet — C# reference implementation
// NuGet: Install-Package FolioProtocol
// Built on: dotnet/Open-XML-SDK (MIT)
// Targets: .NET 8+ (LTS)
//
// This library is the Word/enterprise path.
// Single-file deployment: publish with AOT for folio.exe
// GOOS equivalent: dotnet publish -r win-x64 -c Release
//                  -p:PublishSingleFile=true
//                  -p:PublishAot=true

using System;
using System.Collections.Generic;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.RegularExpressions;
using DocumentFormat.OpenXml.Packaging;

namespace FolioProtocol;

// ─── Data model (FLP-0001) ────────────────────────────────────────────────────

public record FolioIdentity(
    [property: JsonPropertyName("id")]         string Id,
    [property: JsonPropertyName("created")]    string Created,
    [property: JsonPropertyName("created-by")] string CreatedBy,
    [property: JsonPropertyName("title")]      string? Title = null
);

public record FolioVersion(
    [property: JsonPropertyName("v")]           int V,
    [property: JsonPropertyName("author")]      string Author,
    [property: JsonPropertyName("timestamp")]   string Timestamp,
    [property: JsonPropertyName("fingerprint")] string Fingerprint,
    [property: JsonPropertyName("ast-version")] string AstVersion,
    [property: JsonPropertyName("note")]        string? Note = null,
    [property: JsonPropertyName("format")]      string? Format = null
);

public record FolioOp(
    [property: JsonPropertyName("type")] string Type,
    [property: JsonPropertyName("loc")]  string? Loc = null,
    [property: JsonPropertyName("old")]  string? Old = null,
    [property: JsonPropertyName("new")]  string? New = null
);

public record FolioMarkup(
    [property: JsonPropertyName("id")]               string Id,
    [property: JsonPropertyName("from")]             string From,
    [property: JsonPropertyName("submitted")]        string Submitted,
    [property: JsonPropertyName("base-version")]     int BaseVersion,
    [property: JsonPropertyName("base-fingerprint")] string BaseFingerprint,
    [property: JsonPropertyName("status")]           string Status,
    [property: JsonPropertyName("from-display")]     string? FromDisplay = null,
    [property: JsonPropertyName("note")]             string? Note = null,
    [property: JsonPropertyName("incorporated-in")]  int? IncorporatedIn = null,
    [property: JsonPropertyName("ops")]              FolioOp[]? Ops = null
);

public record FolioSignoff(
    [property: JsonPropertyName("id")]                      string Id,
    [property: JsonPropertyName("by")]                      string By,
    [property: JsonPropertyName("timestamp")]               string Timestamp,
    [property: JsonPropertyName("version")]                 int Version,
    [property: JsonPropertyName("fingerprint-at-signoff")]  string FingerprintAtSignoff,
    [property: JsonPropertyName("by-display")]              string? ByDisplay = null,
    [property: JsonPropertyName("scope")]                   string[]? Scope = null
)
{
    public bool IsStale(string currentFingerprint) =>
        FingerprintAtSignoff != currentFingerprint;
}

public record FolioCollaboration(
    [property: JsonPropertyName("pen-holder")] string? PenHolder,
    [property: JsonPropertyName("markups")]    FolioMarkup[] Markups,
    [property: JsonPropertyName("signoffs")]   FolioSignoff[] Signoffs
);

public record FolioRecord(
    [property: JsonPropertyName("folio")]         string Folio,
    [property: JsonPropertyName("identity")]      FolioIdentity Identity,
    [property: JsonPropertyName("history")]       FolioVersion[] History,
    [property: JsonPropertyName("events")]        object[] Events,
    [property: JsonPropertyName("collaboration")] FolioCollaboration Collaboration
)
{
    public int CurrentVersion => History.Length > 0 ? History[^1].V : 0;
    public string? CurrentFingerprint => History.Length > 0 ? History[^1].Fingerprint : null;

    public FolioSignoff[] StaleSignoffs =>
        Collaboration.Signoffs
            .Where(s => s.IsStale(CurrentFingerprint ?? ""))
            .ToArray();
}

// ─── JSON options ─────────────────────────────────────────────────────────────

internal static class JsonOpts
{
    public static readonly JsonSerializerOptions Default = new()
    {
        WriteIndented = true,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        PropertyNamingPolicy = null, // we use explicit JsonPropertyName
    };
}

// ─── FolioDocument — main API ─────────────────────────────────────────────────

/// <summary>
/// Main entry point for the Folio Protocol .NET library.
/// Reads and writes Folio records embedded in DOCX and OCF-based files.
/// Also supports sidecar files for any format.
/// </summary>
public sealed class FolioDocument : IDisposable
{
    private readonly string _documentPath;
    private bool _disposed;

    private FolioDocument(string documentPath)
    {
        _documentPath = documentPath;
    }

    // ── Entry points ──────────────────────────────────────────────────────────

    /// <summary>
    /// Reads the Folio record from a DOCX file.
    /// Returns null if no Folio record exists.
    /// </summary>
    public static FolioRecord? ReadDocx(string path)
    {
        using var doc = WordprocessingDocument.Open(path, isEditable: false);
        return ReadFromDocx(doc);
    }

    /// <summary>
    /// Reads the Folio record from a sidecar .folio file.
    /// Returns null if no sidecar exists.
    /// </summary>
    public static FolioRecord? ReadSidecar(string documentPath)
    {
        var sidecar = documentPath + ".folio";
        if (!File.Exists(sidecar)) return null;

        var json = File.ReadAllText(sidecar, Encoding.UTF8);
        return JsonSerializer.Deserialize<FolioRecord>(json, JsonOpts.Default);
    }

    /// <summary>
    /// Reads from the best available transport for the given file extension.
    /// DOCX: embedded Custom XML Part
    /// ODF/EPUB: embedded META-INF/folio.json
    /// Other: sidecar .folio file
    /// </summary>
    public static FolioRecord? Read(string path)
    {
        var ext = Path.GetExtension(path).ToLowerInvariant();
        return ext switch
        {
            ".docx" => ReadDocx(path),
            ".odt" or ".ods" or ".odp" or ".epub" => ReadOdf(path),
            _ => ReadSidecar(path)
        };
    }

    /// <summary>
    /// Writes a Folio record to a DOCX file's Custom XML Part.
    /// Replaces any existing Folio part.
    /// </summary>
    public static void WriteDocx(string path, FolioRecord record)
    {
        var json = JsonSerializer.Serialize(record, JsonOpts.Default);
        var xmlContent = BuildXmlPart(json);
        var xmlBytes = Encoding.UTF8.GetBytes(xmlContent);

        // Size check per FLP-0005 §2.2
        if (xmlBytes.Length > 900 * 1024)
            throw new InvalidOperationException(
                $"Folio record size {xmlBytes.Length} bytes exceeds 900KB " +
                "Word Online limit. Compaction required.");

        using var doc = WordprocessingDocument.Open(path, isEditable: true);

        // Remove existing Folio parts
        var existing = doc.MainDocumentPart?.CustomXmlParts
            ?.Where(p => IsOurPart(p))
            .ToList();
        existing?.ForEach(p => doc.MainDocumentPart!.DeletePart(p));

        // Add new part
        var part = doc.MainDocumentPart!
            .AddCustomXmlPart(CustomXmlPartType.CustomXml);
        using var stream = part.GetStream(FileMode.Create, FileAccess.Write);
        stream.Write(xmlBytes);

        doc.Save();
    }

    /// <summary>
    /// Writes a Folio record to a sidecar .folio file.
    /// </summary>
    public static void WriteSidecar(string documentPath, FolioRecord record)
    {
        var sidecar = documentPath + ".folio";
        var json = JsonSerializer.Serialize(record, JsonOpts.Default);
        File.WriteAllText(sidecar, json, Encoding.UTF8);
    }

    /// <summary>
    /// Writes using the best transport for the file extension.
    /// </summary>
    public static void Write(string path, FolioRecord record)
    {
        var ext = Path.GetExtension(path).ToLowerInvariant();
        switch (ext)
        {
            case ".docx":
                WriteDocx(path, record);
                break;
            case ".odt" or ".ods" or ".odp" or ".epub":
                WriteOdf(path, record);
                break;
            default:
                WriteSidecar(path, record);
                break;
        }
    }

    // ── Fingerprinting (FLP-0004) ─────────────────────────────────────────────

    /// <summary>
    /// Computes a Folio fingerprint by calling Pandoc as a subprocess.
    /// Returns (fingerprint, astVersion).
    ///
    /// Pandoc must be installed on the system (pandoc.org).
    /// This is a single-binary download with no installer on Windows.
    /// </summary>
    public static (string Fingerprint, string AstVersion) ComputeFingerprint(
        string documentPath)
    {
        var ext = Path.GetExtension(documentPath).ToLowerInvariant();

        // PDF uses content-stream approach (deferred to pdfcpu integration)
        // For now, use pandoc text extraction
        return RunPandocFingerprint(documentPath);
    }

    private static (string, string) RunPandocFingerprint(string path)
    {
        // Run pandoc to get AST JSON
        var psi = new System.Diagnostics.ProcessStartInfo("pandoc")
        {
            Arguments = $"\"{path}\" -t json",
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };

        using var proc = System.Diagnostics.Process.Start(psi)
            ?? throw new InvalidOperationException(
                "Could not start pandoc. Is pandoc installed? See pandoc.org");

        var stdout = proc.StandardOutput.ReadToEnd();
        proc.WaitForExit();

        if (proc.ExitCode != 0)
        {
            var stderr = proc.StandardError.ReadToEnd();
            throw new InvalidOperationException(
                $"pandoc failed on {Path.GetFileName(path)}: {stderr}");
        }

        // Parse AST JSON
        using var doc = JsonDocument.Parse(stdout);
        var root = doc.RootElement;

        // Extract API version
        var apiVersion = "unknown";
        if (root.TryGetProperty("pandoc-api-version", out var versionEl))
        {
            var parts = versionEl.EnumerateArray()
                .Select(x => x.GetInt32().ToString());
            apiVersion = string.Join(".", parts);
        }

        // Strip noise keys before hashing
        var astDict = JsonSerializer.Deserialize<Dictionary<string, JsonElement>>(stdout)!;
        astDict.Remove("pandoc-api-version");
        StripMetaNoise(astDict);

        // Canonical JSON → SHA-256
        var canonical = JsonSerializer.Serialize(
            astDict,
            new JsonSerializerOptions { WriteIndented = false }
        );

        using var sha = SHA256.Create();
        var hashBytes = sha.ComputeHash(Encoding.UTF8.GetBytes(canonical));
        var hex = BitConverter.ToString(hashBytes).Replace("-", "").ToLowerInvariant();

        return ($"sha256:{hex}", apiVersion);
    }

    private static readonly HashSet<string> NoiseMetaKeys = new(StringComparer.OrdinalIgnoreCase)
    {
        "date", "generator", "producer", "modified",
        "revision", "editing-cycles", "creation-date", "template", "last-printed"
    };

    private static void StripMetaNoise(Dictionary<string, JsonElement> ast)
    {
        if (!ast.TryGetValue("meta", out var meta)) return;
        if (meta.ValueKind != JsonValueKind.Object) return;

        var metaDict = JsonSerializer.Deserialize<Dictionary<string, JsonElement>>(
            meta.GetRawText())!;

        foreach (var key in NoiseMetaKeys)
            metaDict.Remove(key);

        ast["meta"] = JsonSerializer.Deserialize<JsonElement>(
            JsonSerializer.Serialize(metaDict));
    }

    /// <summary>
    /// Verifies a document's current content against a stored fingerprint.
    /// Returns true if content matches.
    /// </summary>
    public static bool VerifyFingerprint(
        string documentPath,
        string storedFingerprint,
        string storedAstVersion)
    {
        var (current, currentVersion) = ComputeFingerprint(documentPath);

        if (currentVersion != storedAstVersion)
        {
            Console.Error.WriteLine(
                $"Warning: Pandoc AST version mismatch " +
                $"(stored={storedAstVersion}, current={currentVersion}). " +
                "Fingerprint comparison may be unreliable. See FLP-0004 §5.");
        }

        return current == storedFingerprint;
    }

    // ── ODF Transport (FLP-0011) ──────────────────────────────────────────────

    private static FolioRecord? ReadOdf(string path)
    {
        using var zip = ZipFile.OpenRead(path);
        var entry = zip.GetEntry("META-INF/folio.json");
        if (entry == null) return null;

        using var stream = entry.Open();
        using var reader = new StreamReader(stream, Encoding.UTF8);
        var json = reader.ReadToEnd();
        return JsonSerializer.Deserialize<FolioRecord>(json, JsonOpts.Default);
    }

    private static void WriteOdf(string path, FolioRecord record)
    {
        var json = JsonSerializer.Serialize(record, JsonOpts.Default);
        var jsonBytes = Encoding.UTF8.GetBytes(json);
        var tmp = path + ".folio.tmp";

        // Rewrite the ZIP, replacing META-INF/folio.json
        using (var inZip = ZipFile.OpenRead(path))
        using (var outStream = new FileStream(tmp, FileMode.Create))
        using (var outZip = new ZipArchive(outStream, ZipArchiveMode.Create))
        {
            foreach (var entry in inZip.Entries)
            {
                if (entry.FullName == "META-INF/folio.json") continue;
                var newEntry = outZip.CreateEntry(entry.FullName,
                    CompressionLevel.Optimal);
                using var src = entry.Open();
                using var dst = newEntry.Open();
                src.CopyTo(dst);
            }

            var folioEntry = outZip.CreateEntry("META-INF/folio.json",
                CompressionLevel.Optimal);
            using var folioStream = folioEntry.Open();
            folioStream.Write(jsonBytes);
        }

        File.Move(tmp, path, overwrite: true);
    }

    // ── DOCX Custom XML Part helpers ──────────────────────────────────────────

    private static FolioRecord? ReadFromDocx(WordprocessingDocument doc)
    {
        var parts = doc.MainDocumentPart?.CustomXmlParts;
        if (parts == null) return null;

        foreach (var part in parts)
        {
            if (!IsOurPart(part)) continue;

            using var stream = part.GetStream();
            using var reader = new StreamReader(stream, Encoding.UTF8);
            var content = reader.ReadToEnd();

            // Extract JSON from XML wrapper
            var json = ExtractJsonFromXmlPart(content);
            if (json == null) continue;

            return JsonSerializer.Deserialize<FolioRecord>(json, JsonOpts.Default);
        }

        return null;
    }

    private static bool IsOurPart(CustomXmlPart part)
    {
        try
        {
            using var stream = part.GetStream();
            using var reader = new StreamReader(stream, Encoding.UTF8);
            var content = reader.ReadToEnd();
            return content.Contains("folioprotocol.io");
        }
        catch { return false; }
    }

    private static string BuildXmlPart(string json)
    {
        var escaped = json.Replace("]]>", "]]]]><![CDATA[>");
        return $"""
            <?xml version="1.0" encoding="UTF-8"?>
            <folio xmlns="http://schemas.folioprotocol.io/v1"><![CDATA[{escaped}]]></folio>
            """;
    }

    private static string? ExtractJsonFromXmlPart(string xml)
    {
        var cdataMatch = Regex.Match(xml, @"<!\[CDATA\[([\s\S]*?)\]\]>");
        if (cdataMatch.Success) return cdataMatch.Groups[1].Value;

        var tagMatch = Regex.Match(xml, @"<folio[^>]*>([\s\S]*?)</folio>");
        if (tagMatch.Success) return tagMatch.Groups[1].Value.Trim();

        return null;
    }

    // ── IDisposable ───────────────────────────────────────────────────────────

    public void Dispose()
    {
        if (!_disposed)
        {
            _disposed = true;
        }
    }
}

// ─── Record builder ───────────────────────────────────────────────────────────

/// <summary>
/// Fluent builder for creating new Folio records.
/// </summary>
public sealed class FolioRecordBuilder
{
    private readonly string _author;
    private string? _title;
    private string _note = "Initial version";

    public FolioRecordBuilder(string author) => _author = author;

    public FolioRecordBuilder WithTitle(string title)
    {
        _title = title;
        return this;
    }

    public FolioRecordBuilder WithNote(string note)
    {
        _note = note;
        return this;
    }

    public FolioRecord Build(string documentPath)
    {
        var (fingerprint, astVersion) =
            FolioDocument.ComputeFingerprint(documentPath);

        var now = DateTime.UtcNow.ToString("O");
        var id = $"urn:folio:{Guid.NewGuid():D}";
        var ext = Path.GetExtension(documentPath).ToLowerInvariant();

        return new FolioRecord(
            Folio: "1.0",
            Identity: new FolioIdentity(
                Id: id,
                Created: now,
                CreatedBy: _author,
                Title: _title),
            History: [
                new FolioVersion(
                    V: 1,
                    Author: _author,
                    Timestamp: now,
                    Fingerprint: fingerprint,
                    AstVersion: astVersion,
                    Note: _note,
                    Format: FormatFromExt(ext))
            ],
            Events: [
                new { @event = "INITIALIZED", timestamp = now,
                      by = _author, format = FormatFromExt(ext) }
            ],
            Collaboration: new FolioCollaboration(
                PenHolder: _author,
                Markups: [],
                Signoffs: [])
        );
    }

    private static string FormatFromExt(string ext) => ext switch
    {
        ".docx" => "docx",
        ".odt"  => "odt",
        ".ods"  => "ods",
        ".odp"  => "odp",
        ".pdf"  => "pdf",
        ".md"   => "markdown",
        ".txt"  => "plain",
        ".html" => "html",
        ".epub" => "epub",
        _       => "unknown"
    };
}
