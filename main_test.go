package main

import (
	"os"
	"strings"
	"testing"
)

// Helper function to create a test editor
func createTestEditor(content string) *Editor {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test_*.md")
	if err != nil {
		panic(err)
	}
	defer tmpfile.Close()

	// Write content to file
	if _, err := tmpfile.WriteString(content); err != nil {
		panic(err)
	}

	// Create editor
	editor := &Editor{
		lines:           strings.Split(content, "\n"),
		cursorX:         0,
		cursorY:         0,
		filename:        tmpfile.Name(),
		width:           80,
		height:          25,
		offsetY:         0,
		undoStack:       make([][]string, 0),
		redoStack:       make([][]string, 0),
		modified:        false,
		searchTerm:      "",
		searchIndex:     0,
		truncated:       false,
		maxLines:        100000,
		selectionStart:  false,
		selectionStartX: 0,
		selectionStartY: 0,
		clipboard:       "",
	}

	// Push initial state
	editor.pushUndoState()
	return editor
}

// Cleanup test editor
func cleanupTestEditor(editor *Editor) {
	os.Remove(editor.filename)
}

func TestBasicNavigation(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Test initial position
	if editor.cursorX != 0 || editor.cursorY != 0 {
		t.Errorf("Initial cursor position should be (0,0), got (%d,%d)", editor.cursorX, editor.cursorY)
	}

	// Test moving down
	editor.cursorY = 1
	if editor.cursorY != 1 {
		t.Errorf("Cursor Y should be 1, got %d", editor.cursorY)
	}

	// Test moving right
	editor.cursorX = 3
	if editor.cursorX != 3 {
		t.Errorf("Cursor X should be 3, got %d", editor.cursorX)
	}
}

func TestAutomaticIndentation(t *testing.T) {
	content := "  - Item 1\n  - Item 2"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Position cursor at end of first line
	editor.cursorY = 0
	editor.cursorX = len(editor.lines[0])

	// Insert newline
	editor.insertNewline()

	// Check that indentation is preserved
	if editor.lines[1] != "  " {
		t.Errorf("Expected indentation '  ', got '%s'", editor.lines[1])
	}

	// Check cursor position
	if editor.cursorX != 2 || editor.cursorY != 1 {
		t.Errorf("Cursor should be at (2,1), got (%d,%d)", editor.cursorX, editor.cursorY)
	}
}

func TestWordNavigation(t *testing.T) {
	content := "Hello world test"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Test word navigation right
	editor.moveWordRight()
	if editor.cursorX != 6 { // Should be at beginning of "world"
		t.Errorf("After moveWordRight, cursor should be at 6, got %d", editor.cursorX)
	}

	editor.moveWordRight()
	if editor.cursorX != 12 { // Should be at beginning of "test"
		t.Errorf("After second moveWordRight, cursor should be at 12, got %d", editor.cursorX)
	}

	// Test word navigation left
	editor.moveWordLeft()
	if editor.cursorX != 6 { // Should be at beginning of "world"
		t.Errorf("After moveWordLeft, cursor should be at 6, got %d", editor.cursorX)
	}

	editor.moveWordLeft()
	if editor.cursorX != 0 { // Should be at beginning of "Hello"
		t.Errorf("After second moveWordLeft, cursor should be at 0, got %d", editor.cursorX)
	}
}

func TestTextSelection(t *testing.T) {
	content := "Hello world"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Start selection
	editor.startSelection()
	if !editor.selectionStart {
		t.Error("Selection should be active")
	}

	// Move cursor to create selection
	editor.cursorX = 5
	selected := editor.getSelectedText()
	if selected != "Hello" {
		t.Errorf("Selected text should be 'Hello', got '%s'", selected)
	}

	// Clear selection
	editor.clearSelection()
	if editor.selectionStart {
		t.Error("Selection should be cleared")
	}
}

func TestClipboardOperations(t *testing.T) {
	content := "Hello world"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Select text
	editor.startSelection()
	editor.cursorX = 5

	// Copy
	editor.copy()
	if editor.clipboard != "Hello" {
		t.Errorf("Clipboard should contain 'Hello', got '%s'", editor.clipboard)
	}

	// Move cursor and paste
	editor.cursorX = 11
	editor.clearSelection()
	editor.paste()

	if editor.lines[0] != "Hello worldHello" {
		t.Errorf("After paste, line should be 'Hello worldHello', got '%s'", editor.lines[0])
	}
}

func TestSearch(t *testing.T) {
	content := "Hello world\nThis is a test\nHello again"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Set search term
	editor.searchTerm = "Hello"

	// Find first occurrence (should find the second "Hello" since we start from 0,0)
	editor.findNext()
	if editor.cursorX != 0 || editor.cursorY != 2 {
		t.Errorf("First search should find (0,2), got (%d,%d)", editor.cursorX, editor.cursorY)
	}

	// Find next occurrence (should wrap to first "Hello")
	editor.findNext()
	if editor.cursorX != 0 || editor.cursorY != 0 {
		t.Errorf("Second search should find (0,0), got (%d,%d)", editor.cursorX, editor.cursorY)
	}

	// Find next occurrence (should find second "Hello" again)
	editor.findNext()
	if editor.cursorX != 0 || editor.cursorY != 2 {
		t.Errorf("Third search should find (0,2), got (%d,%d)", editor.cursorX, editor.cursorY)
	}
}

func TestUndoRedo(t *testing.T) {
	content := "Hello"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Make a change (insertChar automatically pushes undo state)
	editor.cursorX = 5
	editor.insertChar('!')

	if editor.lines[0] != "Hello!" {
		t.Errorf("After insert, line should be 'Hello!', got '%s'", editor.lines[0])
	}

	// Undo
	editor.undo()
	if editor.lines[0] != "Hello" {
		t.Errorf("After undo, line should be 'Hello', got '%s'", editor.lines[0])
	}

	// Redo
	editor.redo()
	if editor.lines[0] != "Hello!" {
		t.Errorf("After redo, line should be 'Hello!', got '%s'", editor.lines[0])
	}
}

func TestChunkLoading(t *testing.T) {
	editor := &Editor{
		maxLines: 5, // Set small limit for testing
		lines:    []string{},
	}

	// Create content with more lines than maxLines
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7"

	// Simulate loading with chunk limit
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i >= editor.maxLines {
			editor.truncated = true
			break
		}
		editor.lines = append(editor.lines, line)
	}

	// Check that only maxLines were loaded
	if len(editor.lines) != 5 {
		t.Errorf("Should have loaded 5 lines, got %d", len(editor.lines))
	}

	// Check that truncated flag is set
	if !editor.truncated {
		t.Error("Truncated flag should be set")
	}
}

func TestWordCount(t *testing.T) {
	content := "Hello world\nThis is a test\nWith multiple lines"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	wordCount := editor.wordCount()
	expectedCount := 9 // "Hello", "world", "This", "is", "a", "test", "With", "multiple", "lines"

	if wordCount != expectedCount {
		t.Errorf("Word count should be %d, got %d", expectedCount, wordCount)
	}
}

func TestTextInsertion(t *testing.T) {
	content := "Hello"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Insert character in middle
	editor.cursorX = 2
	editor.insertChar('X')

	if editor.lines[0] != "HeXllo" {
		t.Errorf("After insert, line should be 'HeXllo', got '%s'", editor.lines[0])
	}

	// Check cursor position
	if editor.cursorX != 3 {
		t.Errorf("Cursor should be at position 3, got %d", editor.cursorX)
	}
}

func TestBackspace(t *testing.T) {
	content := "Hello"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Position cursor at end
	editor.cursorX = 5

	// Backspace
	editor.backspace()

	if editor.lines[0] != "Hell" {
		t.Errorf("After backspace, line should be 'Hell', got '%s'", editor.lines[0])
	}

	// Check cursor position
	if editor.cursorX != 4 {
		t.Errorf("Cursor should be at position 4, got %d", editor.cursorX)
	}
}

func TestMultilineOperations(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Test multiline selection
	editor.startSelection()
	editor.cursorY = 1
	editor.cursorX = 4

	selected := editor.getSelectedText()
	expected := "Line 1\nLine"
	if selected != expected {
		t.Errorf("Multiline selection should be '%s', got '%s'", expected, selected)
	}

	// Test multiline paste
	editor.clipboard = "Test\nMultiline"
	editor.cursorY = 0
	editor.cursorX = 0
	editor.clearSelection()
	editor.paste()

	if editor.lines[0] != "Test" {
		t.Errorf("First line after paste should be 'Test', got '%s'", editor.lines[0])
	}
	if editor.lines[1] != "MultilineLine 1" {
		t.Errorf("Second line after paste should be 'MultilineLine 1', got '%s'", editor.lines[1])
	}
}

func TestStatusBarInfo(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	editor := createTestEditor(content)
	defer cleanupTestEditor(editor)

	// Test line count
	if len(editor.lines) != 3 {
		t.Errorf("Should have 3 lines, got %d", len(editor.lines))
	}

	// Test word count
	wordCount := editor.wordCount()
	if wordCount != 6 { // "Line", "1", "Line", "2", "Line", "3"
		t.Errorf("Word count should be 6, got %d", wordCount)
	}

	// Test truncated status
	if editor.truncated {
		t.Error("Small file should not be truncated")
	}
}

func TestEdgeCases(t *testing.T) {
	// Test empty file
	editor := createTestEditor("")
	defer cleanupTestEditor(editor)

	if len(editor.lines) != 1 || editor.lines[0] != "" {
		t.Error("Empty file should have one empty line")
	}

	// Test search in empty file
	editor.searchTerm = "test"
	editor.findNext() // Should not panic

	// Test operations on empty file
	editor.insertChar('a')
	if editor.lines[0] != "a" {
		t.Errorf("After insert in empty file, should be 'a', got '%s'", editor.lines[0])
	}
}
