package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Editor struct {
	screen          tcell.Screen
	lines           []string
	cursorX         int
	cursorY         int
	filename        string
	width           int
	height          int
	offsetY         int
	offsetX         int        // Horizontal scroll offset
	undoStack       [][]string // Stack of previous states of lines
	redoStack       [][]string // Stack of undone states of lines
	modified        bool       // Tracks if the file has unsaved changes
	searchTerm      string     // Current search term
	searchIndex     int        // Current search result index
	truncated       bool       // Whether the file was truncated due to size
	maxLines        int        // Maximum lines to load (10,000 by default)
	selectionStart  bool       // Whether selection is active
	selectionStartX int        // Selection start X position
	selectionStartY int        // Selection start Y position
	clipboard       string     // Internal clipboard for cut/copy/paste
	currentChunk    int        // Current chunk number (0-based)
}

func NewEditor(filename string) (*Editor, error) {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
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
		screen:          screen,
		lines:           []string{""},
		cursorX:         0,
		cursorY:         0,
		filename:        filename,
		width:           width,
		height:          height,
		offsetY:         0,
		offsetX:         0,
		undoStack:       make([][]string, 0),
		redoStack:       make([][]string, 0),
		modified:        false,
		searchTerm:      "",
		searchIndex:     0,
		truncated:       false,
		maxLines:        10000, // Default to 10,000 lines
		selectionStart:  false,
		selectionStartX: 0,
		selectionStartY: 0,
		clipboard:       "",
		currentChunk:    0,
	}

	// Load existing file if it exists
	if err := editor.loadFile(); err != nil {
		// File doesn't exist, that's fine
	}

	return editor, nil
}

func (e *Editor) loadFile() error {
	file, err := os.Open(e.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	e.lines = []string{}
	scanner := bufio.NewScanner(file)
	lineCount := 0

	// Load file with chunk loading to prevent crashes on huge files
	for scanner.Scan() {
		if lineCount >= e.maxLines {
			e.truncated = true
			break
		}
		e.lines = append(e.lines, scanner.Text())
		lineCount++
	}

	if len(e.lines) == 0 {
		e.lines = []string{""}
	}

	e.pushUndoState() // Save initial state after loading
	return scanner.Err()
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

func (e *Editor) pushUndoState() {
	// Make a deep copy of lines to store in undoStack
	linesCopy := make([]string, len(e.lines))
	copy(linesCopy, e.lines)
	e.undoStack = append(e.undoStack, linesCopy)
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

		// Pop and load previous state from undo stack
		e.undoStack = e.undoStack[:len(e.undoStack)-1]
		previousState := e.undoStack[len(e.undoStack)-1]
		e.lines = make([]string, len(previousState))
		copy(e.lines, previousState)

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

	// Ensure cursorX is within bounds for the current line
	currentLineLength := 0
	if len(e.lines) > 0 {
		currentLineLength = len(e.lines[e.cursorY])
	}
	if e.cursorX > currentLineLength {
		e.cursorX = currentLineLength
	}
}

func (e *Editor) scroll() {
	// Vertical scrolling
	if e.cursorY < e.offsetY {
		e.offsetY = e.cursorY
	}
	if e.cursorY >= e.offsetY+e.height-1 {
		e.offsetY = e.cursorY - e.height + 2
	}
	
	// Horizontal scrolling - keep cursor visible with some margin
	margin := 5 // Keep at least 5 characters visible on left/right
	
	// Scroll left if cursor is too far left
	if e.cursorX < e.offsetX+margin {
		e.offsetX = e.cursorX - margin
		if e.offsetX < 0 {
			e.offsetX = 0
		}
	}
	
	// Scroll right if cursor is too far right
	if e.cursorX >= e.offsetX+e.width-margin {
		e.offsetX = e.cursorX - e.width + margin + 1
	}
}

func (e *Editor) wordCount() int {
	count := 0
	for _, line := range e.lines {
		fields := strings.Fields(line) // Splits by whitespace
		count += len(fields)
	}
	return count
}

func (e *Editor) isWordChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func (e *Editor) moveWordLeft() {
	if e.cursorY >= len(e.lines) {
		return
	}

	line := e.lines[e.cursorY]

	// If at beginning of line, move to end of previous line
	if e.cursorX == 0 {
		if e.cursorY > 0 {
			e.cursorY--
			e.cursorX = len(e.lines[e.cursorY])
		}
		return
	}

	// Move left past whitespace
	for e.cursorX > 0 && !e.isWordChar(rune(line[e.cursorX-1])) {
		e.cursorX--
	}

	// Move left past word characters
	for e.cursorX > 0 && e.isWordChar(rune(line[e.cursorX-1])) {
		e.cursorX--
	}
}

func (e *Editor) moveWordRight() {
	if e.cursorY >= len(e.lines) {
		return
	}

	line := e.lines[e.cursorY]

	// If at end of line, move to beginning of next line
	if e.cursorX >= len(line) {
		if e.cursorY < len(e.lines)-1 {
			e.cursorY++
			e.cursorX = 0
		}
		return
	}

	// Move right past word characters
	for e.cursorX < len(line) && e.isWordChar(rune(line[e.cursorX])) {
		e.cursorX++
	}

	// Move right past whitespace
	for e.cursorX < len(line) && !e.isWordChar(rune(line[e.cursorX])) {
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
		searchX := 0
		if y == startY {
			searchX = startX
			// Ensure searchX doesn't exceed line length
			if searchX >= len(line) {
				continue
			}
		}

		if idx := strings.Index(strings.ToLower(line[searchX:]), strings.ToLower(e.searchTerm)); idx != -1 {
			e.cursorY = y
			e.cursorX = searchX + idx
			return
		}
	}

	// If not found, wrap around to beginning
	for y := 0; y < startY; y++ {
		line := e.lines[y]
		if idx := strings.Index(strings.ToLower(line), strings.ToLower(e.searchTerm)); idx != -1 {
			e.cursorY = y
			e.cursorX = idx
			return
		}
	}

	// Check the current line from beginning to cursor
	if startY < len(e.lines) {
		line := e.lines[startY]
		if e.cursorX > 0 && e.cursorX <= len(line) {
			if idx := strings.Index(strings.ToLower(line[:e.cursorX]), strings.ToLower(e.searchTerm)); idx != -1 {
				e.cursorX = idx
				return
			}
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
		if startX > len(line) {
			startX = len(line)
		}
		if endX > len(line) {
			endX = len(line)
		}
		return line[startX:endX]
	}

	// Multi-line selection
	var result strings.Builder
	for y := startY; y <= endY; y++ {
		if y >= len(e.lines) {
			break
		}
		line := e.lines[y]

		if y == startY {
			// First line
			if startX < len(line) {
				result.WriteString(line[startX:])
			}
		} else if y == endY {
			// Last line
			if endX > len(line) {
				endX = len(line)
			}
			result.WriteString(line[:endX])
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
			if startX > len(line) {
				startX = len(line)
			}
			if endX > len(line) {
				endX = len(line)
			}
			e.lines[startY] = line[:startX] + line[endX:]
			e.cursorX = startX
			e.cursorY = startY
		}
	} else {
		// Multi-line deletion
		if startY < len(e.lines) && endY < len(e.lines) {
			// Combine start and end lines
			startLine := e.lines[startY]
			endLine := e.lines[endY]

			if startX > len(startLine) {
				startX = len(startLine)
			}
			if endX > len(endLine) {
				endX = len(endLine)
			}

			newLine := startLine[:startX] + endLine[endX:]

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
		newLine := line[:e.cursorX] + lines[0] + line[e.cursorX:]
		e.lines[e.cursorY] = newLine
		e.cursorX += len(lines[0])
	} else {
		// Multi-line paste
		line := e.lines[e.cursorY]
		firstPart := line[:e.cursorX]
		lastPart := line[e.cursorX:]

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
		e.cursorX = len(lines[len(lines)-1])
	}

	e.modified = true
}

func (e *Editor) saveFile() error {
	if e.currentChunk == 0 && !e.truncated {
		// Simple case: small file or first chunk of non-truncated file
		return e.saveEntireFile()
	}
	
	// Complex case: we're in a chunk of a larger file
	return e.saveChunkToFile()
}

func (e *Editor) saveEntireFile() error {
	file, err := os.Create(e.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i, line := range e.lines {
		if i > 0 {
			writer.WriteString("\n")
		}
		writer.WriteString(line)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	e.modified = false
	return nil
}

func (e *Editor) saveChunkToFile() error {
	// Read the entire original file
	originalFile, err := os.Open(e.filename)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	var allLines []string
	scanner := bufio.NewScanner(originalFile)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Calculate where this chunk starts in the original file
	chunkStartLine := e.currentChunk * e.maxLines
	chunkEndLine := chunkStartLine + e.maxLines
	
	// Clamp to actual file bounds
	if chunkEndLine > len(allLines) {
		chunkEndLine = len(allLines)
	}
	
	// Replace the original chunk with our edited lines
	var newAllLines []string
	
	// Keep everything before our chunk
	newAllLines = append(newAllLines, allLines[:chunkStartLine]...)
	
	// Add our edited chunk
	newAllLines = append(newAllLines, e.lines...)
	
	// Keep everything after our chunk
	newAllLines = append(newAllLines, allLines[chunkEndLine:]...)

	// Write the entire modified file
	file, err := os.Create(e.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i, line := range newAllLines {
		if i > 0 {
			writer.WriteString("\n")
		}
		writer.WriteString(line)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	
	e.modified = false
	return nil
}

func (e *Editor) insertChar(ch rune) {
	e.pushUndoState()
	e.clearSearch()
	if e.cursorY >= len(e.lines) {
		e.lines = append(e.lines, "")
		e.cursorY = len(e.lines) - 1
	}

	line := e.lines[e.cursorY]
	if e.cursorX > len(line) {
		e.cursorX = len(line)
	}

	// Insert character at cursor position
	newLine := line[:e.cursorX] + string(ch) + line[e.cursorX:]
	e.lines[e.cursorY] = newLine
	e.cursorX++
	e.modified = true

	// Handle line wrapping if line exceeds width
	if len(newLine) >= e.width {
		e.wrapCurrentLine()
	}
}

func (e *Editor) wrapCurrentLine() {
	line := e.lines[e.cursorY]
	if len(line) < e.width {
		return
	}

	// Find a good break point (prefer spaces)
	breakPoint := e.width - 1
	for i := e.width - 1; i > e.width-20 && i > 0; i-- {
		if line[i] == ' ' {
			breakPoint = i
			break
		}
	}

	// Split the line
	firstPart := line[:breakPoint]
	secondPart := line[breakPoint:]
	if strings.HasPrefix(secondPart, " ") {
		secondPart = secondPart[1:] // Remove leading space
	}

	e.lines[e.cursorY] = firstPart

	// Insert new line after current
	newLines := make([]string, len(e.lines)+1)
	copy(newLines, e.lines[:e.cursorY+1])
	newLines[e.cursorY+1] = secondPart
	copy(newLines[e.cursorY+2:], e.lines[e.cursorY+1:])
	e.lines = newLines

	// Move cursor to next line if it was at the break point
	if e.cursorX >= breakPoint {
		e.cursorY++
		e.cursorX = e.cursorX - breakPoint
		if e.cursorX < 0 {
			e.cursorX = 0
		}
	}
}

func (e *Editor) insertNewline() {
	e.pushUndoState()
	e.clearSearch()
	if e.cursorY >= len(e.lines) {
		e.lines = append(e.lines, "")
		e.cursorY = len(e.lines) - 1
	}

	line := e.lines[e.cursorY]
	if e.cursorX > len(line) {
		e.cursorX = len(line)
	}

	// Split current line at cursor
	firstPart := line[:e.cursorX]
	secondPart := line[e.cursorX:]

	// Extract leading whitespace from the current line for auto-indentation
	// This preserves indentation for markdown lists, code blocks, etc.
	leadingWhitespace := ""
	for _, char := range line {
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
	e.cursorX = len(leadingWhitespace) // Position cursor after indentation
	e.modified = true
}

func (e *Editor) backspace() {
	e.pushUndoState()
	e.clearSearch()
	if e.cursorX > 0 {
		// Delete character before cursor
		line := e.lines[e.cursorY]
		newLine := line[:e.cursorX-1] + line[e.cursorX:]
		e.lines[e.cursorY] = newLine
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
		e.cursorX = len(prevLine)
		e.modified = true
	}
}

func (e *Editor) delete() {
	e.pushUndoState()
	e.clearSearch()
	if e.cursorY < len(e.lines) {
		line := e.lines[e.cursorY]
		if e.cursorX < len(line) {
			// Delete character at cursor position
			newLine := line[:e.cursorX] + line[e.cursorX+1:]
			e.lines[e.cursorY] = newLine
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
}

func (e *Editor) handleResize() {
	e.width, e.height = e.screen.Size()
	e.screen.Clear()
}

func (e *Editor) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()

	switch ev.Buttons() {
	case tcell.Button1: // Left click
		// Convert screen coordinates to text coordinates (account for horizontal scrolling)
		textY := y + e.offsetY
		textX := x + e.offsetX

		// Validate coordinates and don't allow clicking on status bar
		if textY >= 0 && textY < len(e.lines) && y < e.height-1 {
			e.cursorY = textY
			line := e.lines[textY]
			if textX > len(line) {
				textX = len(line)
			}
			if textX < 0 {
				textX = 0
			}
			e.cursorX = textX
			e.clearSelection()
		}

	case tcell.ButtonNone:
		// Check for scroll wheel events
		if ev.Buttons()&tcell.WheelUp != 0 {
			// Scroll up (3 lines at a time)
			e.offsetY -= 3
			if e.offsetY < 0 {
				e.offsetY = 0
			}
		} else if ev.Buttons()&tcell.WheelDown != 0 {
			// Scroll down (3 lines at a time)
			maxOffset := len(e.lines) - e.height + 1
			if maxOffset < 0 {
				maxOffset = 0
			}
			e.offsetY += 3
			if e.offsetY > maxOffset {
				e.offsetY = maxOffset
			}
		}
	}
}

func (e *Editor) drawLineWithHighlight(line string, startX, y int) {
	// Apply horizontal scrolling - only show the visible portion of the line
	visibleLine := ""
	if e.offsetX < len(line) {
		endX := e.offsetX + e.width
		if endX > len(line) {
			endX = len(line)
		}
		visibleLine = line[e.offsetX:endX]
	}
	
	if e.searchTerm == "" {
		// No search term, draw normally
		for x, ch := range visibleLine {
			if x >= e.width {
				break
			}
			e.screen.SetContent(startX+x, y, ch, nil, tcell.StyleDefault)
		}
		return
	}

	// Draw with search highlighting - case insensitive search
	lowerLine := strings.ToLower(visibleLine)
	lowerSearch := strings.ToLower(e.searchTerm)
	searchLen := len(e.searchTerm)

	x := 0
	for x < len(visibleLine) {
		if x >= e.width {
			break
		}

		// Check if we have a match at this position
		if x+searchLen <= len(visibleLine) && strings.HasPrefix(lowerLine[x:], lowerSearch) {
			// Highlight the search term with yellow background
			highlightStyle := tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
			for i := 0; i < searchLen; i++ {
				if x+i >= e.width {
					break
				}
				ch := rune(visibleLine[x+i])
				e.screen.SetContent(startX+x+i, y, ch, nil, highlightStyle)
			}
			x += searchLen
		} else {
			// Regular character
			ch := rune(visibleLine[x])
			e.screen.SetContent(startX+x, y, ch, nil, tcell.StyleDefault)
			x++
		}
	}
}

func (e *Editor) drawSelection() {
	if !e.selectionStart {
		return
	}

	startX, startY := e.selectionStartX, e.selectionStartY
	endX, endY := e.cursorX, e.cursorY

	// Ensure start comes before end
	if startY > endY || (startY == endY && startX > endX) {
		startX, endX = endX, startX
		startY, endY = endY, startY
	}

	selectionStyle := tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)

	if startY == endY {
		// Single line selection
		screenY := startY - e.offsetY
		if screenY >= 0 && screenY < e.height-1 && startY < len(e.lines) {
			line := e.lines[startY]
			// Clamp coordinates to line bounds
			if startX > len(line) {
				startX = len(line)
			}
			if endX > len(line) {
				endX = len(line)
			}
			
			// Apply selection highlight accounting for horizontal scrolling
			for x := startX; x < endX; x++ {
				screenX := x - e.offsetX
				if screenX >= 0 && screenX < e.width {
					ch := ' '
					if x < len(line) {
						ch = rune(line[x])
					}
					e.screen.SetContent(screenX, screenY, ch, nil, selectionStyle)
				}
			}
		}
	} else {
		// Multi-line selection
		for y := startY; y <= endY; y++ {
			screenY := y - e.offsetY
			if screenY >= 0 && screenY < e.height-1 && y < len(e.lines) {
				line := e.lines[y]
				
				var lineStartX, lineEndX int
				if y == startY {
					lineStartX = startX
					lineEndX = len(line)
				} else if y == endY {
					lineStartX = 0
					lineEndX = endX
				} else {
					lineStartX = 0
					lineEndX = len(line)
				}
				
				// Clamp coordinates to line bounds
				if lineStartX > len(line) {
					lineStartX = len(line)
				}
				if lineEndX > len(line) {
					lineEndX = len(line)
				}
				
				// Apply selection highlight accounting for horizontal scrolling
				for x := lineStartX; x < lineEndX; x++ {
					screenX := x - e.offsetX
					if screenX >= 0 && screenX < e.width {
						ch := ' '
						if x < len(line) {
							ch = rune(line[x])
						}
						e.screen.SetContent(screenX, screenY, ch, nil, selectionStyle)
					}
				}
			}
		}
	}
}

func (e *Editor) draw() {
	e.screen.Clear()

	// Draw all lines, leaving space for status bar
	for y := 0; y < e.height-1; y++ {
		lineIdx := y + e.offsetY
		if lineIdx >= len(e.lines) {
			break
		}
		line := e.lines[lineIdx]
		e.drawLineWithHighlight(line, 0, y)
	}

	// Draw selection highlighting
	e.drawSelection()

	// Draw status bar
	e.drawStatusBar()

	// Position cursor (account for horizontal scrolling)
	screenCursorX := e.cursorX - e.offsetX
	screenCursorY := e.cursorY - e.offsetY
	if screenCursorY < e.height-1 && screenCursorX >= 0 && screenCursorX < e.width {
		e.screen.ShowCursor(screenCursorX, screenCursorY)
	}

	e.screen.Show()
}

func (e *Editor) drawStatusBar() {
	statusStyle := tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite)

	// Clear the status bar line
	for x := 0; x < e.width; x++ {
		e.screen.SetContent(x, e.height-1, ' ', nil, statusStyle)
	}

	filename := filepath.Base(e.filename)
	modified := ""
	if e.modified {
		modified = " [Modified]"
	}
	truncated := ""
	if e.truncated {
		if e.currentChunk > 0 {
			truncated = " [Truncated - Ctrl+T/Ctrl+B]"
		} else {
			truncated = " [Truncated - Ctrl+T for more]"
		}
	} else if e.currentChunk > 0 {
		truncated = " [Chunk view - Ctrl+B for prev]"
	}
	wordCount := e.wordCount()
	status := fmt.Sprintf(" %s%s%s | Ln %d/%d, Col %d | Words: %d", filename, modified, truncated, e.cursorY+1, len(e.lines), e.cursorX+1, wordCount)

	e.drawText(0, e.height-1, status, statusStyle)
}

func (e *Editor) drawText(x, y int, text string, style tcell.Style) {
	for i, r := range text {
		e.screen.SetContent(x+i, y, r, nil, style)
	}
}

func (e *Editor) prompt(prompt string) string {
	// Draw the prompt
	e.drawStatusBar()
	e.drawText(0, e.height-1, prompt, tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite))
	e.screen.Show()

	// Wait for user input
	input := ""
	for {
		ev := e.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEnter:
				return input
			case tcell.KeyEscape:
				return ""
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
			default:
				if ev.Rune() != 0 {
					input += string(ev.Rune())
				}
			}
		}
		// Update the prompt with user input
		e.drawStatusBar()
		e.drawText(0, e.height-1, prompt+input, tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite))
		e.screen.Show()
	}
}

func (e *Editor) run() error {
	defer e.screen.Fini()

	// Initial draw
	e.draw()

	for {
		ev := e.screen.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			// Handle keyboard events - includes standard shortcuts and navigation
			switch ev.Key() {
			case tcell.KeyCtrlD:
				// Save and exit
				if err := e.saveFile(); err != nil {
					return fmt.Errorf("failed to save file: %v", err)
				}
				return nil

			case tcell.KeyCtrlS:
				// Save file
				if err := e.saveFile(); err != nil {
					// Could show error in status bar, but for now just continue
				}

			case tcell.KeyCtrlZ:
				// Undo
				e.undo()

			case tcell.KeyCtrlY:
				// Redo
				e.redo()

			case tcell.KeyCtrlA:
				// Select entire document
				e.selectionStart = true
				e.selectionStartX = 0
				e.selectionStartY = 0
				e.cursorY = len(e.lines) - 1
				if e.cursorY >= 0 {
					e.cursorX = len(e.lines[e.cursorY])
				}

			case tcell.KeyCtrlF:
				// Find/Search
				e.search()

			case tcell.KeyF3:
				// Find next
				e.findNext()

			case tcell.KeyCtrlG:
				// Go to line
				e.goToLine()

			case tcell.KeyCtrlT:
				// Next chunk
				e.loadNextChunk()

			case tcell.KeyCtrlB:
				// Previous chunk (back)
				e.loadPrevChunk()

			case tcell.KeyCtrlX:
				// Cut
				e.cut()

			case tcell.KeyCtrlC:
				// Copy (but check if it's the exit command)
				if e.selectionStart {
					e.copy()
				} else {
					// Original Ctrl+C behavior (exit)
					if e.modified {
						response := e.prompt("Save changes? (y/n): ")
						if response == "y" {
							if err := e.saveFile(); err != nil {
								return fmt.Errorf("failed to save file: %v", err)
							}
						}
					}
					return nil
				}

			case tcell.KeyCtrlV:
				// Paste
				e.paste()

			case tcell.KeyEnter:
				e.insertNewline()

			case tcell.KeyBackspace, tcell.KeyBackspace2:
				e.backspace()

			case tcell.KeyDelete:
				e.delete()

			case tcell.KeyTab:
				// Insert 4 spaces for tab
				for i := 0; i < 4; i++ {
					e.insertChar(' ')
				}
			case tcell.KeyLeft:
				// Handle Left arrow with modifier keys (Ctrl=word nav, Shift=selection)
				if ev.Modifiers()&tcell.ModCtrl != 0 {
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					e.moveWordLeft()
				} else {
					// Regular left arrow movement
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					if e.cursorX > 0 {
						e.cursorX--
					} else if e.cursorY > 0 {
						e.cursorY--
						e.cursorX = len(e.lines[e.cursorY])
					}
				}

			case tcell.KeyRight:
				// Check if Ctrl is pressed for word navigation
				if ev.Modifiers()&tcell.ModCtrl != 0 {
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					e.moveWordRight()
				} else {
					// Check if Shift is pressed for selection
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					if e.cursorY < len(e.lines) && e.cursorX < len(e.lines[e.cursorY]) {
						e.cursorX++
					} else if e.cursorY < len(e.lines)-1 {
						e.cursorY++
						e.cursorX = 0
					}
				}

			case tcell.KeyHome:
				// Check if Ctrl is pressed for document start
				if ev.Modifiers()&tcell.ModCtrl != 0 {
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					e.cursorY = 0
					e.cursorX = 0
				} else {
					// Regular Home - go to beginning of line
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					e.cursorX = 0
				}

			case tcell.KeyEnd:
				// Check if Ctrl is pressed for document end
				if ev.Modifiers()&tcell.ModCtrl != 0 {
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					e.cursorY = len(e.lines) - 1
					if e.cursorY >= 0 {
						e.cursorX = len(e.lines[e.cursorY])
					}
				} else {
					// Regular End - go to end of line
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					if e.cursorY < len(e.lines) {
						e.cursorX = len(e.lines[e.cursorY])
					}
				}

			case tcell.KeyPgUp:
				e.clearSelection()
				e.cursorY -= e.height - 1
				if e.cursorY < 0 {
					e.cursorY = 0
				}
			case tcell.KeyPgDn:
				e.clearSelection()
				e.cursorY += e.height - 1
				if e.cursorY >= len(e.lines) {
					e.cursorY = len(e.lines) - 1
				}

			case tcell.KeyUp:
				// Check if Shift is pressed for selection
				if ev.Modifiers()&tcell.ModShift != 0 {
					e.startSelection()
				} else {
					e.clearSelection()
				}
				if e.cursorY > 0 {
					e.cursorY--
					if e.cursorX > len(e.lines[e.cursorY]) {
						e.cursorX = len(e.lines[e.cursorY])
					}
				}

			case tcell.KeyDown:
				// Check if Shift is pressed for selection
				if ev.Modifiers()&tcell.ModShift != 0 {
					e.startSelection()
				} else {
					e.clearSelection()
				}
				if e.cursorY < len(e.lines)-1 {
					e.cursorY++
					if e.cursorX > len(e.lines[e.cursorY]) {
						e.cursorX = len(e.lines[e.cursorY])
					}
				}

			default:
				// Regular character input
				if ev.Rune() != 0 && ev.Rune() >= 32 {
					e.clearSelection()
					e.insertChar(ev.Rune())
				}
			}

		case *tcell.EventResize:
			e.handleResize()

		case *tcell.EventMouse:
			e.handleMouse(ev)
		}

		e.scroll()
		e.draw()
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <filename>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nA modern minimal text editor.\n")
		fmt.Fprintf(os.Stderr, "Write in a distraction-free environment with modern conveniences.\n")
		fmt.Fprintf(os.Stderr, "\nKey features:\n")
		fmt.Fprintf(os.Stderr, "  - Automatic indentation for markdown\n")
		fmt.Fprintf(os.Stderr, "  - Standard shortcuts (Ctrl+S, Ctrl+Z/Y, Ctrl+X/C/V)\n")
		fmt.Fprintf(os.Stderr, "  - Word navigation (Ctrl+Arrow keys)\n")
		fmt.Fprintf(os.Stderr, "  - Text selection (Shift+Arrow keys)\n")
		fmt.Fprintf(os.Stderr, "  - Search with highlighting (Ctrl+F)\n")
		fmt.Fprintf(os.Stderr, "  - Mouse support and large file handling\n")
		fmt.Fprintf(os.Stderr, "\nBasic controls:\n")
		fmt.Fprintf(os.Stderr, "  Ctrl+D  Save and exit\n")
		fmt.Fprintf(os.Stderr, "  Ctrl+C  Exit without saving\n")
		fmt.Fprintf(os.Stderr, "  Ctrl+T  Next chunk (prompts to save if modified)\n")
		fmt.Fprintf(os.Stderr, "  Ctrl+B  Previous chunk (prompts to save if modified)\n")
		fmt.Fprintf(os.Stderr, "\nLarge file behavior:\n")
		fmt.Fprintf(os.Stderr, "  - Files >10K lines are split into 10K line chunks\n")
		fmt.Fprintf(os.Stderr, "  - Chunks show fixed line ranges (e.g., 1-10K, 10K-20K)\n")
		fmt.Fprintf(os.Stderr, "  - Edits may cause content to 'spill' into adjacent chunks\n")
		os.Exit(1)
	}

	filename := os.Args[1]

	editor, err := NewEditor(filename)
	if err != nil {
		log.Fatalf("Failed to create editor: %v", err)
	}

	if err := editor.run(); err != nil {
		log.Fatalf("Editor error: %v", err)
	}
}
