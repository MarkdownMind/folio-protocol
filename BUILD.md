# Folio Protocol — Build and Deployment

## folio-go (Primary CLI — recommended)

### Prerequisites
- Go 1.22+: https://go.dev/dl/
- Pandoc: https://pandoc.org/installing.html (single binary, no installer)

### Build

```bash
cd folio-go

# Windows .exe — no runtime required on target machine
GOOS=windows GOARCH=amd64 go build -o folio.exe ./cmd/folio

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o folio ./cmd/folio

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o folio ./cmd/folio

# Linux
GOOS=linux GOARCH=amd64 go build -o folio ./cmd/folio
```

### Deploy to Windows (law firm)

```
1. Build folio.exe on any machine with Go installed
2. Copy folio.exe to target Windows machine
3. Copy pandoc.exe to same machine (or system PATH)
4. IT: whitelist folio.exe and pandoc.exe
5. Done — no installer, no runtime, no admin rights needed for use
```

### Verify the build

```bash
folio version
# folio 1.0.0-alpha
# Pandoc: 3.2.1
# Protocol: https://github.com/folioprotocol/spec
```

---

## folio-dotnet (Word Add-in / NuGet / Enterprise)

### Prerequisites
- .NET 8 SDK: https://dotnet.microsoft.com/download
- Open XML SDK: automatically via NuGet

### Build

```bash
cd folio-dotnet

# Restore dependencies
dotnet restore

# Build library
dotnet build -c Release

# Pack for NuGet
dotnet pack -c Release -o ./nupkg

# AOT single-file .exe for CLI use
dotnet publish -c Release -r win-x64 \
  -p:PublishSingleFile=true \
  -p:PublishAot=true \
  -o ./publish/win-x64
```

### NuGet usage

```bash
dotnet add package FolioProtocol
```

```csharp
using FolioProtocol;

// Initialize tracking
var record = new FolioRecordBuilder("ian@firm.com")
    .WithTitle("Service Agreement — Acme Corp")
    .WithNote("Initial draft")
    .Build("contract.docx");

FolioDocument.WriteDocx("contract.docx", record);

// Read and verify
var existing = FolioDocument.ReadDocx("contract.docx");
var verified = FolioDocument.VerifyFingerprint(
    "contract.docx",
    existing!.CurrentFingerprint!,
    existing.History[^1].AstVersion
);

Console.WriteLine(verified ? "✓ Content verified" : "⚠ Content changed");
```

---

## folio-js (Word Add-in TypeScript)

### Prerequisites
- Node.js 20+
- Office.js (via Microsoft CDN or npm)
- Yeoman generator for Office add-ins (for scaffolding)

### Build

```bash
cd folio-js
npm install
npm run build
# Outputs: dist/folio-core.js
```

### Office Add-in integration

```typescript
import {
  initializeRecord,
  readRecord,
  saveVersion,
  getStaleSignoffs,
  isWordOnline,
  canWrite
} from '@folioprotocol/office-js';

// In your task pane:
Office.onReady(async () => {

  // Check platform capability
  if (isWordOnline()) {
    showReadOnlyWarning(); // FLP-0005 §2.1
  }

  // Load existing record
  const record = await readRecord();

  if (!record) {
    // Not tracking — offer to start
    showInitializeButton();
  } else {
    // Show history, sign-offs, pending markups
    renderSidebar(record);

    // Check for stale sign-offs
    const stale = getStaleSignoffs(record);
    if (stale.length > 0) {
      showStaleWarning(stale);
    }
  }

  // Hook into save events
  Office.context.document.addHandlerAsync(
    Office.EventType.DocumentSelectionChanged,
    async () => {
      if (canWrite()) {
        const newV = await saveVersion(currentUser);
        if (newV) updateVersionDisplay(newV);
      }
    }
  );
});
```

### AppSource deployment

```
1. Register at Partner Center: partner.microsoft.com
2. Create manifest.xml (see /folio-js/manifest.xml)
3. Host add-in at HTTPS endpoint (your server or Azure Static Web Apps)
4. Required live before submission:
   - Privacy policy URL
   - Support URL
   - EULA URL
5. Submit to AppSource
6. Review: 3-5 business days (budget 4 weeks for first submission)
7. Approved: IT deploys org-wide via M365 Admin Center
   No user install required — appears in Word ribbon automatically
```

---

## Pandoc — the one system dependency

Pandoc is the only external dependency across all implementations.
It is free, open source, and available as a single binary with no installer.

### Windows
```
Download: https://github.com/jgm/pandoc/releases/latest
File: pandoc-{version}-windows-x86_64.msi (or .zip)
The .zip version needs no installer — just extract and add to PATH
```

### macOS
```bash
brew install pandoc
# or download directly from pandoc.org
```

### Linux
```bash
apt install pandoc          # Debian/Ubuntu
dnf install pandoc          # Fedora
# or download from pandoc.org for latest version
```

### For enterprise deployment (Windows)
```
1. Download pandoc-{version}-windows-x86_64.zip
2. Extract pandoc.exe
3. Place in same directory as folio.exe OR add to system PATH
4. IT whitelist: pandoc.exe (signed by pandoc.org)
```

---

## Dependencies summary

| Component      | External deps          | Runtime needed?       |
|----------------|------------------------|----------------------|
| folio-go CLI   | pandoc (system binary) | No — single .exe     |
| folio-dotnet   | pandoc + Open XML SDK  | No — AOT single file |
| folio-js       | pandoc (via server)    | Node.js for dev only |
| pandoc itself  | None                   | No — single binary   |

**Total user-facing dependencies: pandoc.exe + folio.exe**
Both are single binaries. Both are whitelistable by IT.
Neither requires an installer or admin rights to run.
