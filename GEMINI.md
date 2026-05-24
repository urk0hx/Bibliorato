# Bibliorato

Bibliorato is a Go-based Digital Forensics and Incident Response (DFIR) tool designed for analyzing email evidence. It provides a graphical user interface (built with [Fyne](https://fyne.io/)) to parse, view, and enrich email data from various formats.

## Project Overview

- **Core Functionality:** Parses email archive files (MSG, PST, OST, MBOX, EML) and extracts metadata, bodies, and attachments.
- **Architecture:** 
    - `main.go`: Entry point and UI management.
    - `core/`: Defines core data structures (`Evidence`, `Attachment`, `Enrichment`) and the `Plugin` interface.
    - `database/`: Manages a temporary SQLite database (using `modernc.org/sqlite`) for structured storage and retrieval of evidence and enrichments.
    - `parser/`: Contains format-specific parsers for `.msg`, `.pst`, `.ost`, `.mbox`, and `.eml` files.
    - `plugins/`: A registry-based plugin system for data enrichment (e.g., `traceroute.go` for header analysis, `auth_analyzer.go` for SPF/DKIM/DMARC status, and `link_mismatch.go` for deceptive link detection).
- **Key Technologies:** Go, Fyne (GUI), SQLite (pure Go driver), various email parsing libraries (`go-pst`, `go-mbox`, `go-message`).

## Building and Running

### Prerequisites

- [Go](https://go.dev/doc/install) (version 1.26.3 or later as per `go.mod`)
- Fyne dependencies (see [Fyne setup guide](https://developer.fyne.io/started/))

### Commands

- **Run the application:**
  ```bash
  go run main.go
  ```
- **Build the application:**
  ```bash
  go build -o Bibliorato .
  ```
- **Run tests:**
  ```bash
  go test ./...
  ```

## Development Conventions

- **Layered Integrity:** The database schema strictly separates pristine evidence (raw metadata/content) from enrichments (generated analysis data).
- **Sanitization:** All UI-displayed strings should be sanitized using `core.Sanitize` to remove non-printable characters.
- **Plugin System:** New analysis features should be implemented as plugins by satisfying the `core.Plugin` interface and registering them in an `init()` function using `core.RegisterPlugin`.
- **Parser Registry:** New file formats are supported by creating a new file in the `parser/` directory and registering it via an `init()` function using `parser.RegisterStoreParser` (for archives) or `parser.RegisterFileParser` (for standalone emails). The UI will automatically detect and support the new extension.
- **Error Handling:** Use `fmt.Errorf` with wrapping or specific error libraries used in the project (like `github.com/rotisserie/eris`) for descriptive error reporting.
- **UI State:** The UI uses a custom tabbed interface on top of Fyne's `AppTabs` to manage multiple open emails and enrichment views.

## Project Structure

- `_data/`: (Assumed) Storage for sample data or test files.
- `core/`: Fundamental types and plugin infrastructure.
- `database/`: SQLite schema and data access layer.
- `parser/`: File format specific extraction logic.
- `plugins/`: Analytical enrichment modules.
- `vendor/`: Project dependencies.
