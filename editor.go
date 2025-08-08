package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

const maxUndoStates = 100 // Maximum number of undo states to keep in memory

type Editor struct {
	screen      tcell.Screen
	lines       []string
	cursorX     int
	cursorY     int
	filename    string
	width       int
	height      int
	offsetY     int
	offsetX     int        // Horizontal scroll offset
	undoStack   [][]string // Stack of previous states of lines
	redoStack   [][]string // Stack of undone states of lines
	modified    bool       // Tracks if the file has unsaved changes
	searchTerm  string     // Current search term
	searchIndex int        // Current search result index
	// Chunking fields
	truncated          bool   // Whether the file was truncated due to size
	maxLines           int    // Maximum lines to load (10,000 by default)
	selectionStart     bool   // Whether selection is active
	selectionStartX    int    // Selection start X position
	selectionStartY    int    // Selection start Y position
	clipboard          string // Internal clipboard for cut/copy/paste
	currentChunk       int    // Current chunk number (0-based)
	cachedWordCount    int    // Cached word count for performance
	wordCountValid     bool   // Whether cached word count is valid
	scrollAcceleration int    // For smoother trackpad scrolling
	// Momentum scrolling fields
	scrollMomentum    float64 // Current scroll momentum
	maxScrollMomentum float64 // Maximum momentum to prevent runaway scrolling (200-300 lines)
	momentumDecay     float64 // Decay rate per update (0.9 means 10% decay per frame)
}

// Unicode utility functions for rune-aware string operations

// runeLen returns the number of runes in a string
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// runeSubstring extracts a substring by rune positions (start inclusive, end exclusive)
func runeSubstring(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}

	runes := []rune(s)
	if start >= len(runes) {
		return ""
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

// runeInsert inserts a string at a specific rune position
func runeInsert(s string, pos int, insert string) string {
	runes := []rune(s)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

	insertRunes := []rune(insert)
	result := make([]rune, len(runes)+len(insertRunes))
	copy(result, runes[:pos])
	copy(result[pos:], insertRunes)
	copy(result[pos+len(insertRunes):], runes[pos:])
	return string(result)
}

// runeDelete deletes runes from start to end position (end exclusive)
func runeDelete(s string, start, end int) string {
	runes := []rune(s)
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return s
	}

	result := make([]rune, len(runes)-(end-start))
	copy(result, runes[:start])
	copy(result[start:], runes[end:])
	return string(result)
}

// displayWidth returns the display width of a string considering CJK characters
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// displayWidthRune returns the display width of a single rune
func displayWidthRune(r rune) int {
	return runewidth.RuneWidth(r)
}

// runeIndexToByteIndex converts a rune index to byte index in a string
func runeIndexToByteIndex(s string, runeIndex int) int {
	if runeIndex <= 0 {
		return 0
	}

	byteIndex := 0
	currentRune := 0
	for byteIndex < len(s) && currentRune < runeIndex {
		_, size := utf8.DecodeRuneInString(s[byteIndex:])
		byteIndex += size
		currentRune++
	}
	return byteIndex
}

// byteIndexToRuneIndex converts a byte index to rune index in a string
func byteIndexToRuneIndex(s string, byteIndex int) int {
	if byteIndex <= 0 {
		return 0
	}
	if byteIndex >= len(s) {
		return utf8.RuneCountInString(s)
	}

	return utf8.RuneCountInString(s[:byteIndex])
}

// isWordRune checks if a rune is part of a word (Unicode-aware)
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func NewEditor(filename string) (*Editor, error) {
	// Ensure directory exists only if filename is provided
	if filename != "" {
		dir := filepath.Dir(filename)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %v", err)
		}
	}

	// Initialize screen
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	if err := screen.Init(); err != nil {
		return nil, err
	}

	// Enable mouse support
	screen.EnableMouse()

	// Get initial dimensions
	width, height := screen.Size()

	editor := &Editor{
		screen:      screen,
		lines:       []string{""},
		cursorX:     0,
		cursorY:     0,
		filename:    filename,
		width:       width,
		height:      height,
		offsetY:     0,
		offsetX:     0,
		undoStack:   make([][]string, 0),
		redoStack:   make([][]string, 0),
		modified:    false,
		searchTerm:  "",
		searchIndex: 0,
		// Chunking fields
		truncated:          false,
		maxLines:           10000, // Default to 10,000 lines
		selectionStart:     false,
		selectionStartX:    0,
		selectionStartY:    0,
		clipboard:          "",
		currentChunk:       0,
		cachedWordCount:    0,
		wordCountValid:     false,
		scrollAcceleration: 0,
		// Momentum scrolling initialization
		scrollMomentum:    0.0,
		maxScrollMomentum: 250.0, // Cap at 250 lines of momentum
		momentumDecay:     0.85,  // 15% decay per frame for smooth deceleration
	}

	// Load existing file if filename is provided and file exists
	if filename != "" {
		if err := editor.loadFile(); err != nil {
			// File doesn't exist, that's fine
		}
	}

	return editor, nil
}

// saveFileWithPrompt handles saving the file, prompting for filename if needed
func (e *Editor) saveFileWithPrompt() error {
	if e.filename == "" {
		filename := e.promptFilename("Save as", "")
		if filename == "" {
			return nil // User cancelled
		}

		// Check if file exists and ask for confirmation
		if _, err := os.Stat(filename); err == nil {
			// File exists, ask for confirmation
			if !e.promptYesNo(fmt.Sprintf("File '%s' exists. Overwrite?", filepath.Base(filename))) {
				return nil // User chose not to overwrite
			}
		}

		e.filename = filename

		// Ensure directory exists for new filename
		dir := filepath.Dir(e.filename)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
	}
	return e.saveFile()
}


func (e *Editor) pushUndoState() {
	// Make a deep copy of lines to store in undoStack
	linesCopy := make([]string, len(e.lines))
	copy(linesCopy, e.lines)
	e.undoStack = append(e.undoStack, linesCopy)

	// Limit undo stack size to prevent unbounded memory growth
	if len(e.undoStack) > maxUndoStates {
		// Remove oldest state (first element)
		e.undoStack = e.undoStack[1:]
	}

	// Clear redo stack when a new action is performed
	e.redoStack = [][]string{}
}

func (e *Editor) undo() {
	if len(e.undoStack) > 1 {
		// Save current state (what we're moving away from) to redo stack
		// This allows us to redo this change later
		currentLines := make([]string, len(e.lines))
		copy(currentLines, e.lines)
		e.redoStack = append(e.redoStack, currentLines)

		// Limit redo stack size as well
		if len(e.redoStack) > maxUndoStates {
			e.redoStack = e.redoStack[1:]
		}

		// Pop and load previous state from undo stack
		e.undoStack = e.undoStack[:len(e.undoStack)-1]
		previousState := e.undoStack[len(e.undoStack)-1]
		e.lines = make([]string, len(previousState))
		copy(e.lines, previousState)
		e.invalidateWordCount()

		e.modified = true
		// Adjust cursor position if necessary
		e.adjustCursorPosition()
	}
}

func (e *Editor) redo() {
	if len(e.redoStack) > 0 {
		// Pop state from redo stack and move it back to undo stack
		// This restores the state that was previously undone
		nextState := e.redoStack[len(e.redoStack)-1]
		e.redoStack = e.redoStack[:len(e.redoStack)-1]
		e.undoStack = append(e.undoStack, nextState)

		// Load the state
		e.lines = make([]string, len(nextState))
		copy(e.lines, nextState)
		e.invalidateWordCount()

		e.modified = true
		// Adjust cursor position if necessary
		e.adjustCursorPosition()
	}
}

func (e *Editor) adjustCursorPosition() {
	// Ensure cursorY is within bounds
	if e.cursorY >= len(e.lines) {
		e.cursorY = len(e.lines) - 1
		if e.cursorY < 0 {
			e.cursorY = 0 // Handle empty file case
		}
	}

	// Ensure cursorX is within bounds for the current line (using rune count)
	currentLineLength := 0
	if len(e.lines) > 0 && e.cursorY < len(e.lines) {
		currentLineLength = runeLen(e.lines[e.cursorY])
	}
	if e.cursorX > currentLineLength {
		e.cursorX = currentLineLength
	}
}

func (e *Editor) scroll() {
	// Clamp scroll offsets to valid ranges
	if e.offsetY < 0 {
		e.offsetY = 0
	}
	if e.offsetX < 0 {
		e.offsetX = 0
	}
}

func (e *Editor) invalidateWordCount() {
	e.wordCountValid = false
}

func (e *Editor) wordCount() int {
	if e.wordCountValid {
		return e.cachedWordCount
	}

	count := 0
	for _, line := range e.lines {
		fields := strings.Fields(line) // Splits by whitespace
		count += len(fields)
	}

	e.cachedWordCount = count
	e.wordCountValid = true
	return count
}

func (e *Editor) isWordChar(ch rune) bool {
	return isWordRune(ch)
}

func (e *Editor) moveWordLeft() {
	if e.cursorY >= len(e.lines) {
		return
	}

	line := e.lines[e.cursorY]
	runes := []rune(line)

	// If at beginning of line, move to end of previous line
	if e.cursorX == 0 {
		if e.cursorY > 0 {
			e.cursorY--
			e.cursorX = runeLen(e.lines[e.cursorY])
		}
		return
	}

	// Move left past whitespace
	for e.cursorX > 0 && !e.isWordChar(runes[e.cursorX-1]) {
		e.cursorX--
	}

	// Move left past word characters
	for e.cursorX > 0 && e.isWordChar(runes[e.cursorX-1]) {
		e.cursorX--
	}
}

func (e *Editor) moveWordRight() {
	if e.cursorY >= len(e.lines) {
		return
	}

	line := e.lines[e.cursorY]
	runes := []rune(line)
	lineLen := len(runes)

	// If at end of line, move to beginning of next line
	if e.cursorX >= lineLen {
		if e.cursorY < len(e.lines)-1 {
			e.cursorY++
			e.cursorX = 0
		}
		return
	}

	// Move right past word characters
	for e.cursorX < lineLen && e.isWordChar(runes[e.cursorX]) {
		e.cursorX++
	}

	// Move right past whitespace
	for e.cursorX < lineLen && !e.isWordChar(runes[e.cursorX]) {
		e.cursorX++
	}
}

func (e *Editor) clearSearch() {
	e.searchTerm = ""
}

func (e *Editor) findNext() {
	if e.searchTerm == "" {
		return
	}

	// Start searching from current position
	startY := e.cursorY
	startX := e.cursorX + 1

	// Search forward from current position
	for y := startY; y < len(e.lines); y++ {
		line := e.lines[y]
		lineRunes := []rune(line)
		searchX := 0
		if y == startY {
			searchX = startX
			// Ensure searchX doesn't exceed line length
			if searchX >= len(lineRunes) {
				continue
			}
		}

		searchText := string(lineRunes[searchX:])
		if idx := strings.Index(strings.ToLower(searchText), strings.ToLower(e.searchTerm)); idx != -1 {
			// Convert byte index back to rune index
			runeIdx := utf8.RuneCountInString(searchText[:idx])
			e.cursorY = y
			e.cursorX = searchX + runeIdx
			e.ensureCursorVisible()
			return
		}
	}

	// If not found, wrap around to beginning
	for y := 0; y < startY; y++ {
		line := e.lines[y]
		if idx := strings.Index(strings.ToLower(line), strings.ToLower(e.searchTerm)); idx != -1 {
			// Convert byte index to rune index
			e.cursorY = y
			e.cursorX = utf8.RuneCountInString(line[:idx])
			e.ensureCursorVisible()
			return
		}
	}

	// Check the current line from beginning to cursor
	if startY < len(e.lines) {
		line := e.lines[startY]
		lineRunes := []rune(line)
		if e.cursorX > 0 && e.cursorX <= len(lineRunes) {
			searchText := string(lineRunes[:e.cursorX])
			if idx := strings.Index(strings.ToLower(searchText), strings.ToLower(e.searchTerm)); idx != -1 {
				e.cursorX = utf8.RuneCountInString(searchText[:idx])
				e.ensureCursorVisible()
				return
			}
		}
	}
}

// findPrev moves the cursor to the previous occurrence of the current search term,
// wrapping to the end if necessary.
func (e *Editor) findPrev() {
	if e.searchTerm == "" {
		return
	}

	startY := e.cursorY
	startX := e.cursorX - 1

	// Search backward from current position
	for y := startY; y >= 0; y-- {
		line := e.lines[y]
		lineRunes := []rune(line)
		searchEnd := len(lineRunes)
		if y == startY {
			if startX < 0 {
				continue
			}
			if startX >= len(lineRunes) {
				searchEnd = len(lineRunes)
			} else {
				searchEnd = startX + 1
			}
		}

		segment := string(lineRunes[:searchEnd])
		lowerSeg := strings.ToLower(segment)
		lowerSearch := strings.ToLower(e.searchTerm)
		if idx := strings.LastIndex(lowerSeg, lowerSearch); idx != -1 {
			// Convert byte index back to rune index
			runeIdx := utf8.RuneCountInString(segment[:idx])
			e.cursorY = y
			e.cursorX = runeIdx
			e.ensureCursorVisible()
			return
		}
	}

	// Wrap: search from bottom up to original line
	for y := len(e.lines) - 1; y > startY; y-- {
		line := e.lines[y]
		lowerLine := strings.ToLower(line)
		lowerSearch := strings.ToLower(e.searchTerm)
		if idx := strings.LastIndex(lowerLine, lowerSearch); idx != -1 {
			e.cursorY = y
			e.cursorX = utf8.RuneCountInString(line[:idx])
			e.ensureCursorVisible()
			return
		}
	}
}

func (e *Editor) search() {
	searchTerm := e.prompt("Search: ")
	if searchTerm == "" {
		return
	}

	e.searchTerm = searchTerm
	e.findNext()
}

// searchIncremental provides an interactive, incremental search.
// As the user types, matches are highlighted and the cursor jumps to the next match.
func (e *Editor) searchIncremental() {
	// Seed with the current term so F4 can refine an existing search
	input := []rune(e.searchTerm)
	style := tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)

	redraw := func(resetToFirst bool) {
		e.searchTerm = string(input)
		// When term changes, reset to first occurrence
		if resetToFirst && e.searchTerm != "" {
			e.cursorY = 0
			e.cursorX = -1 // so findNext starts from index 0
			e.findNext()
		}
		// Redraw full screen to show highlights
		e.draw()
		// Overlay the prompt
		prompt := "Search (inc): " + e.searchTerm
		e.drawText(0, e.height-1, prompt, style)
		e.screen.Show()
	}

	redraw(true)

	for {
		ev := e.screen.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventKey:
			switch tev.Key() {
			case tcell.KeyTAB:
				if tev.Modifiers()&tcell.ModShift != 0 {
					e.findPrev()
				} else {
					e.findNext()
				}
				// Keep prompt visible
				e.draw() // redraw full screen to update highlights/cursor
				prompt := "Search (inc): " + string(input)
				e.drawText(0, e.height-1, prompt, style)
				e.screen.Show()
			case tcell.KeyBacktab:
				// Shift+Tab often comes as KeyBacktab
				e.findPrev()
				e.draw()
				prompt := "Search (inc): " + string(input)
				e.drawText(0, e.height-1, prompt, style)
				e.screen.Show()
			case tcell.KeyEscape:
				// Clear highlights and exit
				e.clearSearch()
				e.draw()
				return
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
				redraw(true)
			case tcell.KeyF3:
				// Find next occurrence
				e.findNext()
				// Keep prompt visible
				e.draw()
				prompt := "Search (inc): " + string(input)
				e.drawText(0, e.height-1, prompt, style)
				e.screen.Show()
			case tcell.KeyRune:
				// Regular typed character extends the term
				input = append(input, tev.Rune())
				redraw(true)
			default:
				// ignore others
			}
		}
	}
}

func (e *Editor) goToLine() {
	lineStr := e.prompt("Go to line: ")
	if lineStr == "" {
		return
	}

	// Try to parse the line number
	var lineNum int
	if _, err := fmt.Sscanf(lineStr, "%d", &lineNum); err != nil {
		return // Invalid number, just return
	}

	// Convert to 0-based indexing and validate
	lineNum-- // Convert from 1-based to 0-based
	if lineNum < 0 {
		lineNum = 0
	}
	if lineNum >= len(e.lines) {
		lineNum = len(e.lines) - 1
	}

	// Move cursor to the line
	e.cursorY = lineNum
	e.cursorX = 0
	e.clearSelection()
	e.ensureCursorVisible()
}

func (e *Editor) startSelection() {
	if !e.selectionStart {
		e.selectionStart = true
		e.selectionStartX = e.cursorX
		e.selectionStartY = e.cursorY
	}
}

func (e *Editor) clearSelection() {
	e.selectionStart = false
}

func (e *Editor) getSelectedText() string {
	if !e.selectionStart {
		return ""
	}

	startX, startY := e.selectionStartX, e.selectionStartY
	endX, endY := e.cursorX, e.cursorY

	// Ensure start comes before end
	if startY > endY || (startY == endY && startX > endX) {
		startX, endX = endX, startX
		startY, endY = endY, startY
	}

	if startY == endY {
		// Single line selection
		if startY >= len(e.lines) {
			return ""
		}
		line := e.lines[startY]
		lineRunes := []rune(line)
		if startX > len(lineRunes) {
			startX = len(lineRunes)
		}
		if endX > len(lineRunes) {
			endX = len(lineRunes)
		}
		return string(lineRunes[startX:endX])
	}

	// Multi-line selection
	var result strings.Builder
	for y := startY; y <= endY; y++ {
		if y >= len(e.lines) {
			break
		}
		line := e.lines[y]
		lineRunes := []rune(line)

		if y == startY {
			// First line
			if startX < len(lineRunes) {
				result.WriteString(string(lineRunes[startX:]))
			}
		} else if y == endY {
			// Last line
			if endX > len(lineRunes) {
				endX = len(lineRunes)
			}
			result.WriteString(string(lineRunes[:endX]))
		} else {
			// Middle lines
			result.WriteString(line)
		}

		if y < endY {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (e *Editor) deleteSelection() {
	if !e.selectionStart {
		return
	}

	e.pushUndoState()
	e.clearSearch()
	e.invalidateWordCount()

	startX, startY := e.selectionStartX, e.selectionStartY
	endX, endY := e.cursorX, e.cursorY

	// Ensure start comes before end
	if startY > endY || (startY == endY && startX > endX) {
		startX, endX = endX, startX
		startY, endY = endY, startY
	}

	if startY == endY {
		// Single line deletion
		if startY < len(e.lines) {
			line := e.lines[startY]
			lineRunes := []rune(line)
			if startX > len(lineRunes) {
				startX = len(lineRunes)
			}
			if endX > len(lineRunes) {
				endX = len(lineRunes)
			}
			newRunes := make([]rune, len(lineRunes)-(endX-startX))
			copy(newRunes, lineRunes[:startX])
			copy(newRunes[startX:], lineRunes[endX:])
			e.lines[startY] = string(newRunes)
			e.cursorX = startX
			e.cursorY = startY
		}
	} else {
		// Multi-line deletion
		if startY < len(e.lines) && endY < len(e.lines) {
			// Combine start and end lines
			startLine := e.lines[startY]
			endLine := e.lines[endY]
			startRunes := []rune(startLine)
			endRunes := []rune(endLine)

			if startX > len(startRunes) {
				startX = len(startRunes)
			}
			if endX > len(endRunes) {
				endX = len(endRunes)
			}

			newLine := string(startRunes[:startX]) + string(endRunes[endX:])

			// Remove lines between start and end
			newLines := make([]string, len(e.lines)-(endY-startY))
			copy(newLines, e.lines[:startY])
			newLines[startY] = newLine
			copy(newLines[startY+1:], e.lines[endY+1:])

			e.lines = newLines
			e.cursorX = startX
			e.cursorY = startY
		}
	}

	e.clearSelection()
	e.modified = true
}

func (e *Editor) copy() {
	if !e.selectionStart {
		return
	}
	e.clipboard = e.getSelectedText()
}

func (e *Editor) cut() {
	if !e.selectionStart {
		return
	}
	e.clipboard = e.getSelectedText()
	e.deleteSelection()
}

func (e *Editor) paste() {
	if e.clipboard == "" {
		return
	}

	e.pushUndoState()
	e.clearSearch()

	// If there's a selection, delete it first
	if e.selectionStart {
		e.deleteSelection()
	}

	// Insert clipboard content
	lines := strings.Split(e.clipboard, "\n")
	if len(lines) == 1 {
		// Single line paste
		line := e.lines[e.cursorY]
		newLine := runeInsert(line, e.cursorX, lines[0])
		e.lines[e.cursorY] = newLine
		e.cursorX += runeLen(lines[0])
	} else {
		// Multi-line paste
		line := e.lines[e.cursorY]
		lineRunes := []rune(line)
		firstPart := string(lineRunes[:e.cursorX])
		lastPart := string(lineRunes[e.cursorX:])

		// Create new lines array
		newLines := make([]string, len(e.lines)+len(lines)-1)
		copy(newLines, e.lines[:e.cursorY])

		// Insert first line
		newLines[e.cursorY] = firstPart + lines[0]

		// Insert middle lines
		for i := 1; i < len(lines)-1; i++ {
			newLines[e.cursorY+i] = lines[i]
		}

		// Insert last line
		newLines[e.cursorY+len(lines)-1] = lines[len(lines)-1] + lastPart

		// Copy remaining lines
		copy(newLines[e.cursorY+len(lines):], e.lines[e.cursorY+1:])

		e.lines = newLines
		e.cursorY += len(lines) - 1
		e.cursorX = runeLen(lines[len(lines)-1])
	}

	e.modified = true
	e.ensureCursorVisible()
}

func (e *Editor) insertChar(ch rune) {
	e.pushUndoState()
	e.clearSearch()
	e.invalidateWordCount()
	if e.cursorY >= len(e.lines) {
		e.lines = append(e.lines, "")
		e.cursorY = len(e.lines) - 1
	}

	line := e.lines[e.cursorY]
	lineRunes := []rune(line)
	if e.cursorX > len(lineRunes) {
		e.cursorX = len(lineRunes)
	}

	// Insert character at cursor position using rune-aware operation
	e.lines[e.cursorY] = runeInsert(line, e.cursorX, string(ch))
	e.cursorX++
	e.modified = true
	e.ensureCursorVisible()
}

func (e *Editor) insertNewline() {
	e.pushUndoState()
	e.clearSearch()
	e.invalidateWordCount()
	if e.cursorY >= len(e.lines) {
		e.lines = append(e.lines, "")
		e.cursorY = len(e.lines) - 1
	}

	line := e.lines[e.cursorY]
	lineRunes := []rune(line)
	if e.cursorX > len(lineRunes) {
		e.cursorX = len(lineRunes)
	}

	// Split current line at cursor using rune positions
	firstPart := string(lineRunes[:e.cursorX])
	secondPart := string(lineRunes[e.cursorX:])

	// Extract leading whitespace from the current line for auto-indentation
	// This preserves indentation for markdown lists, code blocks, etc.
	leadingWhitespace := ""
	for _, char := range lineRunes {
		if char == ' ' || char == '\t' {
			leadingWhitespace += string(char)
		} else {
			break
		}
	}

	e.lines[e.cursorY] = firstPart

	// Insert new line with preserved indentation
	newLines := make([]string, len(e.lines)+1)
	copy(newLines, e.lines[:e.cursorY+1])
	newLines[e.cursorY+1] = leadingWhitespace + secondPart
	copy(newLines[e.cursorY+2:], e.lines[e.cursorY+1:])
	e.lines = newLines

	e.cursorY++
	e.cursorX = runeLen(leadingWhitespace) // Position cursor after indentation
	e.modified = true
	e.ensureCursorVisible()
}

func (e *Editor) backspace() {
	e.pushUndoState()
	e.clearSearch()
	e.invalidateWordCount()
	if e.cursorX > 0 {
		// Delete character before cursor using rune-aware operation
		line := e.lines[e.cursorY]
		e.lines[e.cursorY] = runeDelete(line, e.cursorX-1, e.cursorX)
		e.cursorX--
		e.modified = true
	} else if e.cursorY > 0 {
		// Join with previous line
		prevLine := e.lines[e.cursorY-1]
		currentLine := e.lines[e.cursorY]
		e.lines[e.cursorY-1] = prevLine + currentLine

		// Remove current line
		newLines := make([]string, len(e.lines)-1)
		copy(newLines, e.lines[:e.cursorY])
		copy(newLines[e.cursorY:], e.lines[e.cursorY+1:])
		e.lines = newLines

		e.cursorY--
		e.cursorX = runeLen(prevLine)
		e.modified = true
	}
	e.ensureCursorVisible()
}

func (e *Editor) delete() {
	e.pushUndoState()
	e.clearSearch()
	e.invalidateWordCount()
	if e.cursorY < len(e.lines) {
		line := e.lines[e.cursorY]
		lineRunes := []rune(line)
		if e.cursorX < len(lineRunes) {
			// Delete character at cursor position using rune-aware operation
			e.lines[e.cursorY] = runeDelete(line, e.cursorX, e.cursorX+1)
			e.modified = true
		} else if e.cursorY < len(e.lines)-1 {
			// At end of line, join with next line
			nextLine := e.lines[e.cursorY+1]
			e.lines[e.cursorY] = line + nextLine

			// Remove next line
			newLines := make([]string, len(e.lines)-1)
			copy(newLines, e.lines[:e.cursorY+1])
			copy(newLines[e.cursorY+1:], e.lines[e.cursorY+2:])
			e.lines = newLines
			e.modified = true
		}
	}
	e.ensureCursorVisible()
}

func (e *Editor) handleResize() {
	e.width, e.height = e.screen.Size()
	e.screen.Clear()
}

// addScrollMomentum adds momentum from mouse wheel events, capped to prevent runaway scrolling
func (e *Editor) addScrollMomentum(delta float64) {
	e.scrollMomentum += delta

	// Cap momentum to prevent excessive scrolling
	if e.scrollMomentum > e.maxScrollMomentum {
		e.scrollMomentum = e.maxScrollMomentum
	} else if e.scrollMomentum < -e.maxScrollMomentum {
		e.scrollMomentum = -e.maxScrollMomentum
	}
}

// applyScrollMomentum applies accumulated scroll momentum with decay
func (e *Editor) applyScrollMomentum() {
	if e.scrollMomentum == 0 {
		return
	}

	// Apply momentum to scroll position
	if e.scrollMomentum > 0.1 {
		// Scroll down
		scrollAmount := int(e.scrollMomentum * 0.1) // Apply 10% of momentum per frame
		if scrollAmount < 1 {
			scrollAmount = 1
		}

		e.offsetY += scrollAmount

		// Apply file limits
		maxOffset := len(e.lines) - e.height + 1
		if maxOffset < 0 {
			maxOffset = 0
		}
		if e.offsetY > maxOffset {
			e.offsetY = maxOffset
			e.scrollMomentum = 0
		}

	} else if e.scrollMomentum < -0.1 {
		// Scroll up
		scrollAmount := int(-e.scrollMomentum * 0.1)
		if scrollAmount < 1 {
			scrollAmount = 1
		}

		e.offsetY -= scrollAmount
		if e.offsetY < 0 {
			e.offsetY = 0
			e.scrollMomentum = 0 // Stop momentum when hitting bounds
		}
	}

	// Decay momentum
	e.scrollMomentum *= e.momentumDecay

	// Stop momentum when it gets very small
	if e.scrollMomentum < 0.1 && e.scrollMomentum > -0.1 {
		e.scrollMomentum = 0
	}
}

func (e *Editor) loadNextChunk() error {
	if !e.truncated {
		return nil // No more chunks if file wasn't truncated
	}

	// Check if current chunk has unsaved changes
	if e.modified {
		response := e.prompt("Save changes? (y/n): ")
		if response == "y" {
			if err := e.saveFile(); err != nil {
				return fmt.Errorf("failed to save file: %v", err)
			}
		}
		// If "n", continue and lose changes (same as Ctrl+C behavior)
	}

	file, err := os.Open(e.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	// Skip lines to get to the next chunk
	skipLines := (e.currentChunk + 1) * e.maxLines
	for lineCount < skipLines && scanner.Scan() {
		lineCount++
	}

	// Load the next chunk
	e.lines = []string{}
	chunkLines := 0
	hasMoreContent := false

	for scanner.Scan() && chunkLines < e.maxLines {
		e.lines = append(e.lines, scanner.Text())
		chunkLines++
	}

	// Check if there's more content after this chunk
	if scanner.Scan() {
		hasMoreContent = true
	}

	if len(e.lines) == 0 {
		return nil // No more content
	}

	e.currentChunk++
	e.truncated = hasMoreContent

	// Reset cursor to top
	e.cursorX = 0
	e.cursorY = 0
	e.offsetY = 0
	e.offsetX = 0
	e.clearSelection()
	e.clearSearch()

	e.pushUndoState()
	return scanner.Err()
}

func (e *Editor) loadPrevChunk() error {
	if e.currentChunk == 0 {
		return nil // Already at first chunk
	}

	// Check if current chunk has unsaved changes
	if e.modified {
		response := e.prompt("Save changes? (y/n): ")
		if response == "y" {
			if err := e.saveFile(); err != nil {
				return fmt.Errorf("failed to save file: %v", err)
			}
		}
		// If "n", continue and lose changes (same as Ctrl+C behavior)
	}

	file, err := os.Open(e.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	// Skip lines to get to the previous chunk
	skipLines := (e.currentChunk - 1) * e.maxLines
	for lineCount < skipLines && scanner.Scan() {
		lineCount++
	}

	// Load the previous chunk
	e.lines = []string{}
	chunkLines := 0

	for scanner.Scan() && chunkLines < e.maxLines {
		e.lines = append(e.lines, scanner.Text())
		chunkLines++
	}

	if len(e.lines) == 0 {
		e.lines = []string{""}
	}

	e.currentChunk--
	e.truncated = true // If we can go back, there's always more content

	// Reset cursor to top
	e.cursorX = 0
	e.cursorY = 0
	e.offsetY = 0
	e.offsetX = 0
	e.clearSelection()
	e.clearSearch()

	e.pushUndoState()
	return scanner.Err()
}
