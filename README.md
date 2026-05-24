# Bibliorato

Bibliorato is an all-in-one, extensible Digital Forensics and Incident Response (DFIR) tool built in Go. It provides a native, cross-platform graphical user interface (powered by [Fyne](https://fyne.io/)) specifically designed for analyzing, triaging, and enriching email evidence.

## Features
- **Multi-Format Support:** Natively parses individual email files and email store archives.
- **Data Enrichment:** Its default-included plugins analyze email content (authentication headers, deceptive links, routing hops) to surface actionable insights.
- **Layered Integrity:** Utilizes an underlying SQLite database to strictly separate raw evidence from generated analytical data.
- **High-Performance UI:** Offloads heavy markdown parsing and text sanitization to background goroutines, keeping the UI responsive even when loading massive email payloads.

---

## Getting Started

You can download the latest release from the [GitHub releases page](https://github.com/yourusername/Bibliorato/releases) or follow the instructions below to compile from source.

### Prerequisites
- [Go](https://go.dev/doc/install) 1.26.3 or higher.
- C compiler (required by the SQLite driver and Fyne).
- Graphics drivers supported by Fyne (OpenGL). See the [Fyne prerequisites guide](https://developer.fyne.io/started/).

### Compilation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/Bibliorato.git
   cd Bibliorato
   ```

2. Download dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o Bibliorato .
   ```

4. Run the application:
   ```bash
   ./Bibliorato
   ```

---

## Usage

The application divides file loading into two distinct workflows accessible from the **Analyzer** menu:

1. **Open Mail Store... (`.pst`, `.ost`, `.mbox`, others)**
   - Opens email store archives.
   - **Note:** This acts as a "New Session". It will wipe the temporary database and load the entire structure of the mail archive into the Triage List and Folder Tree.

2. **Add Mail File... (`.eml`, `.msg`, others)**
   - Opens individual email files.
   - **Note:** This appends the file to your existing session. The email will be added to the "Loaded Evidence" folder without clearing your previously loaded data, allowing you to load multiple standalone files into a single investigation view.

You can toggle the left folder tree view at any time using the hamburger menu icon in the tab bar. To export your current session, use **Analyzer -> Live Export DB...** to save the SQLite database for external analysis.

---

## Extending Bibliorato

Bibliorato is built to be modular. You can easily add support for new file formats and new analytical plugins without modifying the core UI code.

### Adding a New Plugin (Enricher)
Plugins process raw evidence and output structured CSV data to the UI.
1. Create a new Go file in the `plugins/` directory (e.g., `my_plugin.go`).
2. Implement the `core.Plugin` interface:
   ```go
   type Plugin interface {
       Name() string
       Type() string
       TargetFormat() string
       Description() string
       Process(evidence *Evidence) ([]Enrichment, error)
   }
   ```
3. Register your plugin using an `init()` function:
   ```go
   func init() {
       core.RegisterPlugin(&MyPlugin{})
   }
   ```

### Adding a New File Parser
The application uses a registry pattern to decouple the UI from the parsing logic.
1. Create a new Go file in the `parser/` directory (e.g., `ost.go`).
2. Write a parser function matching either `StoreParser` (for archives) or `FileParser` (for standalone files):
   - **StoreParser:** `func(filepath string) (map[string][]string, string, error)`
   - **FileParser:** `func(filepath string) (*core.Evidence, error)`
3. Register the parser in an `init()` function:
   ```go
   // For store archives:
   func init() {
       parser.RegisterStoreParser(".ost", ParseOSTAndStore)
   }
   
   // OR for standalone files:
   func init() {
       parser.RegisterFileParser(".newext", ParseNewExtAndStore)
   }
   ```
The UI will automatically recognize the new extension and add it to the file picker dialogs.

---

## License

This project is licensed under the [Apache License 2.0](LICENSE) - see the LICENSE file for details.
