# mkmd - A Modern Minimal text Editor

A distraction-free terminal-based text editor built in Go. Think of it as a modern take on `ed` - simple, fast, and focused on the essential writing experience.

## Features

### Core Writing Experience
- **Distraction-free writing** - Clean terminal interface with minimal UI
- **Automatic indentation** - Preserves leading whitespace for nested lists and code blocks
- **Horizontal scrolling** - Navigate through long lines seamlessly with automatic cursor-based scrolling
- **Word count & live status** - Real-time feedback on your writing progress

### Modern Navigation
- **Word-based movement** - `Ctrl+Left/Right` for efficient navigation
- **Home/End keys** - Quick line navigation with `Home/End`, document navigation with `Ctrl+Home/End`
- **Go-to-line** - Jump to any line number with `Ctrl+G`
- **Page navigation** - Scroll by screen with `Page Up/Down`

### Text Selection & Editing
- **Visual text selection** - `Shift+arrows` for precise text selection with blue highlighting
- **Word selection** - `Ctrl+Shift+arrows` for word-based selection
- **Select all** - `Ctrl+A` selects entire document
- **Standard clipboard** - Familiar `Ctrl+X/C/V` for cut/copy/paste (Ctrl+C copies when text is selected)
- **Forward/backward delete** - Both `Backspace` and `Delete` keys supported
- **Undo/redo** - Full `Ctrl+Z/Y` support (bounded history for performance)

### Search & Navigation
- **Search with highlighting** - `Ctrl+F` for search with visual yellow highlighting
- **Case-insensitive search** - Find text regardless of case
- **Wrap-around search** - `F3` to find next occurrence with seamless wrapping
- **Smart highlight clearing** - Search highlights automatically clear when you start editing

### Modern Conveniences
- **Mouse support** - Click to position cursor, scroll wheel for navigation
- **Large file handling** - Intelligently loads files in 10K line chunks with navigation support
- **File auto-detection** - Automatically creates directories as needed
- **Standard shortcuts** - All the shortcuts you expect: `Ctrl+S`, `Ctrl+A`, etc.

## Installation

```bash
go build -o mkmd .
```

## Usage

```bash
# Open an existing file
./mkmd filename.md

# Or launch with an empty buffer
./mkmd
```

there is also binary in the bin folder. The one that has no specification is the macOS one.

## Keyboard Shortcuts

### File Operations
- `Ctrl+D` - Save and exit
- `Ctrl+S` - Save file
- `Ctrl+C` - Copy (if text selected) or Exit (if no selection)

### Navigation
- `Arrow keys` - Move cursor
- `Ctrl+Left/Right` - Jump by words
- `Home/End` - Beginning/end of line
- `Ctrl+Home/End` - Beginning/end of document
- `Page Up/Down` - Scroll by screen
- `Ctrl+A` - Select entire document
- `Ctrl+G` - Go to line number
- `Ctrl+T` - Next chunk (prompts to save if modified)
- `Ctrl+B` - Previous chunk (prompts to save if modified)

### Text Selection
- `Shift+Arrow keys` - Select text
- `Ctrl+Shift+Left/Right` - Select by words
- `Shift+Home/End` - Select to beginning/end of line
- `Ctrl+Shift+Home/End` - Select to beginning/end of document

### Editing
- `Ctrl+Z` - Undo
- `Ctrl+Y` - Redo
- `Ctrl+X` - Cut selected text
- `Ctrl+C` - Copy selected text
- `Ctrl+V` - Paste text
- `Backspace` - Delete character before cursor
- `Delete` - Delete character at cursor
- `Tab` - Insert 4 spaces
- `Enter` - New line with automatic indentation

### Search
- `Ctrl+F` - Find text (with yellow highlighting)
- `F3` - Find next occurrence
- Search highlights clear automatically when editing

### Mouse Support
- **Click** - Position cursor (works with horizontally scrolled content)
- **Scroll wheel** - Scroll up/down

## Long Line Handling

mkmd features intelligent horizontal scrolling for long lines:

- **Automatic scrolling** - View automatically scrolls left/right to keep cursor visible
- **Smart margins** - Maintains 5-character buffer on each side for comfortable editing
- **Seamless navigation** - Use arrow keys normally; scrolling happens automatically
- **Full line access** - Navigate through lines of any length without content being cut off
- **Mouse compatibility** - Click anywhere on long lines with accurate positioning

## Status Bar

The status bar shows:
- Filename and modification status
- Current line/total lines and column position
- Word count
- `[Truncated]` indicator for large files

Example: `test.md [Modified] | Ln 15/42, Col 8 | Words: 127`

## Design Philosophy

mkmd embraces the Unix philosophy of doing one thing well. It's designed for:

- **Writers** who want distraction-free markdown editing
- **Developers** who need quick note-taking and documentation
- **Anyone** who prefers keyboard-driven workflows

The editor maintains the simplicity of classic terminal editors while adding modern conveniences that enhance the writing flow without adding complexity.

## Large File Handling

For files exceeding 10,000 lines, mkmd loads content in chunks to maintain responsiveness. Use `Ctrl+T` to navigate to the next chunk and `Ctrl+B` to go back to the previous chunk. If you have unsaved changes, you'll be prompted to save before navigation. The status bar shows navigation hints and indicates when a file is chunked.

**Note:** Chunks use fixed 10K line boundaries. Large edits may cause content to "spill" into adjacent chunks when navigating.

## Dependencies

- Go 1.24.5 or later
- [tcell](https://github.com/gdamore/tcell) for terminal handling

## Project Structure

- `main.go` — minimal CLI entrypoint that parses the filename and launches the editor
- `editor.go` — core editor state and behaviors (cursor, buffers, word movement, selection, undo/redo, scrolling)
- `input.go` — keyboard and mouse handling, including movement, editing, search, chunk navigation
- `render.go` — rendering pipeline (lines, selection, status bar) and prompts
  - Horizontal scrolling uses display columns, so wide glyphs (e.g., CJK) align correctly
  - Prompts are Unicode-aware; backspace deletes full runes
- `file.go` — file I/O, including loading and chunked saving for large files
- `mkmd_test.go` — comprehensive tests for chunking, Unicode-aware operations, selection, search, scrolling, and prompts
- `bin/` — prebuilt binaries (platform-specific)
- `test-text-files/` — sample large/text fixtures used during development

## Development

- Build: `go build -o mkmd .`
- Run: `./mkmd filename.md`
- Tests: `go test ./...`
- Race detector: `go test -race ./...`
- Lint/vet: `go vet ./...`

Notes:
- Large files (>10k lines) load in 10k-line chunks. Use `Ctrl+T`/`Ctrl+B` to navigate chunks.
- Saving in chunked mode writes the current chunk back into the full file while preserving other chunks.
- Status bar shows filename, modification status, line/column, word count, and chunk/truncated hints.

## 📄 License

under ☕️, check out [the-coffee-license](https://github.com/codinganovel/The-Coffee-License)

I've included both licenses with the repo, do what you know is right. The licensing works by assuming you're operating under good faith.