module github.com/folioprotocol/folio-go

go 1.22

require (
	// Zero external dependencies for core protocol.
	// Pandoc is a system binary, called via os/exec.
	// archive/zip, crypto/sha256, encoding/json all from stdlib.
)
