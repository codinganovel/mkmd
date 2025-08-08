# mkmd Behavior Guide

This document describes all observable behaviors of mkmd and exactly how to trigger them.

## Launch & Files

- Empty buffer: Run `./mkmd` with no arguments to open a new, unnamed buffer.
- Open file: Run `./mkmd <path>` to load an existing file. If it does not exist, an empty buffer with that filename is used on first save.
- Auto-create directories: When saving to a new path, any missing directories in the path are created.

## Saving & Exiting

- Save: `Ctrl+S`
  - If the buffer has no filename, a prompt appears at the status bar: "Save as: ".
  - If a filename exists, the file is written immediately.
- Save and exit: `Ctrl+D`
- Quit: `Ctrl+Q`
  - If the buffer is modified, a prompt appears: "Save changes? (y/n):".
  - `y` saves (prompting for filename if needed), then exits; `n` exits without saving.

## Prompts

- Status-bar prompts: Input appears on the bottom line. Prompts include:
  - Save as: (on first save)
  - Search: (classic search)
  - Search (inc): (incremental search)
  - Go to line: (line number)
- Prompt input is Unicode-aware: backspace deletes a full rune, not a byte.

### Filename Prompt (Save as)

- Type the desired filename and press Enter to save
- Press Escape to cancel
- If the file already exists, you'll be asked to confirm overwrite

## Editing

- Insert character: Type any printable character.
- Newline: `Enter`
  - Inserts a line break and preserves indentation (leading spaces) from the previous line.
- Tab: `Tab` inserts 4 spaces.
- Backspace: `Backspace`
  - Deletes the rune before the cursor; at start of a line, joins with previous line.
- Delete: `Delete`
  - Deletes the rune at the cursor; at end of a line, joins with next line.
- Cut: `Ctrl+X` (if selection exists)
- Copy: `Ctrl+C` (if selection exists)
- Paste: `Ctrl+V`
- Undo: `Ctrl+Z` (history is bounded for performance)
- Redo: `Ctrl+Y`

Note: Whenever you modify text, any active search highlights are cleared automatically.

## Selection

- Start and extend selection with Shift + movement keys. Selection is shown with a blue background.
  - `Shift+Left/Right/Up/Down`
  - `Shift+Home/End`
  - `Ctrl+Shift+Left/Right` (word-based movement while selecting)
- Select all: `Ctrl+A`

## Movement

- Arrow keys: `Left/Right/Up/Down`
- Word movement: `Ctrl+Left` (previous word), `Ctrl+Right` (next word)
- Line start/end: `Home`, `End`
- Document start/end: `Ctrl+Home`, `Ctrl+End`
- Page movement: `Page Up`, `Page Down`
- Go to line: `Ctrl+G`, then type a 1-based line number and press `Enter`.

## Search

All search matches are highlighted in yellow.

- Classic search: `Ctrl+F`
  - Status bar prompt: "Search: "
  - Type search term and press `Enter` to jump to the first match after the cursor; `F3` jumps to the next match.
- Incremental search: `F4`
  - Status bar prompt: "Search (inc): "
  - Type to update search term; the view jumps to the first match of the new term.
  - Navigate matches during incremental search:
    - `Tab`: next match
    - `Shift+Tab` (or `Backtab`): previous match
    - `F3`: next match
  - `Backspace`: remove last rune from the term and jump to the first match of the new term.
  - `Esc`: exit incremental search and clear highlights.

## Mouse

- Click: Position the cursor at the clicked location (Unicode-aware and horizontal-scroll aware).
- Scroll wheel up/down: Smooth vertical scrolling with momentum.
- Trackpad horizontal scroll (or wheel left/right): Adjust horizontal offset.

## Horizontal Scrolling & Long Lines

- Horizontal scrolling is display-width based and Unicode-aware (CJK/wide runes render with correct width).
- Smart margin: The editor maintains a small (~5 columns) horizontal buffer around the cursor. When moving near edges, the viewport auto-adjusts to keep the cursor comfortably visible.

## Status Bar

The bottom line shows:

- Filename, plus "[Modified]" when there are unsaved changes
- Line/total lines and column (1-based)
- Word count
- Chunking hints (see below)

Example: `notes.md [Modified] | Ln 15/42, Col 8 | Words: 127`

## Large Files (Chunking)

- When loading files over 10,000 lines, mkmd loads content in 10,000-line chunks to stay responsive.
- The status bar shows when content is truncated and how to navigate chunks.
- Navigate chunks:
  - Next chunk: `Ctrl+T`
  - Previous chunk: `Ctrl+B`
- If the current chunk has unsaved changes and you switch chunks, mkmd prompts to save.
- Saving in chunked mode updates the corresponding segment of the original file while leaving other chunks intact.


## Rendering

- Unicode-aware rendering: Characters are measured and drawn by display width (e.g., CJK characters and emoji).
- Selection highlight: blue background; Search highlights: yellow background.
- The UI fully redraws on input, resize, and search navigation to ensure the highlights and cursor are current.

## Limits & Notes

- Undo/redo history is bounded to conserve memory during long editing sessions.
- A very long single logical line is supported up to an internal scanner buffer limit (editing remains stable; extremely long lines may be truncated by I/O limits).


