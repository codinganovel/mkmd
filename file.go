package main

import (
	"bufio"
	"os"
)

func (e *Editor) loadFile() error {
	file, err := os.Open(e.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	e.lines = []string{}
	scanner := bufio.NewScanner(file)
	// Increase the scanner buffer to handle very long lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB per line cap
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)
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
	e.invalidateWordCount()
	return scanner.Err()
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
