package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// buildVisualSegments is removed - no longer needed for horizontal scrolling

// advanceToDisplayOffset moves a rune index forward until the given display-column
// offset is consumed. If the offset falls inside a wide rune, the function draws
// the remaining hidden part as blanks and returns the next rune index and updated
// display X.
func (e *Editor) advanceToDisplayOffset(runes []rune, y, startX, offsetCols int) (startRuneIdx, displayX int) {
	displayX = startX
	startRuneIdx = 0
	colOffset := offsetCols

	for startRuneIdx < len(runes) && colOffset > 0 {
		w := displayWidthRune(runes[startRuneIdx])
		if colOffset >= w {
			colOffset -= w
			startRuneIdx++
			continue
		}
		blanks := w - colOffset
		for i := 0; i < blanks && displayX < e.width; i++ {
			e.screen.SetContent(displayX, y, ' ', nil, tcell.StyleDefault)
			displayX++
		}
		colOffset = 0
		startRuneIdx++
		break
	}
	return startRuneIdx, displayX
}

// drawPlainRun draws runes starting at runeIdx until the row fills.
func (e *Editor) drawPlainRun(runes []rune, runeIdx, y, displayX int) {
	for runeIdx < len(runes) && displayX < e.width {
		ch := runes[runeIdx]
		e.screen.SetContent(displayX, y, ch, nil, tcell.StyleDefault)
		displayX += displayWidthRune(ch)
		runeIdx++
	}
}

// drawWithSearchHighlight draws runes with search-term highlighting starting at runeIdx.
func (e *Editor) drawWithSearchHighlight(line string, runes []rune, runeIdx, y, displayX int) {
	lowerLine := strings.ToLower(line)
	lowerSearch := strings.ToLower(e.searchTerm)
	searchRunes := []rune(e.searchTerm)
	searchLen := len(searchRunes)

	for runeIdx < len(runes) && displayX < e.width {
		if searchLen > 0 && runeIdx+searchLen <= len(runes) {
			matchStart := runeIndexToByteIndex(line, runeIdx)
			matchEnd := runeIndexToByteIndex(line, runeIdx+searchLen)
			if matchStart < len(lowerLine) && matchEnd <= len(lowerLine) &&
				strings.HasPrefix(lowerLine[matchStart:], lowerSearch) {
				style := tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
				for i := 0; i < searchLen && runeIdx+i < len(runes) && displayX < e.width; i++ {
					ch := runes[runeIdx+i]
					e.screen.SetContent(displayX, y, ch, nil, style)
					displayX += displayWidthRune(ch)
				}
				runeIdx += searchLen
				continue
			}
		}

		ch := runes[runeIdx]
		e.screen.SetContent(displayX, y, ch, nil, tcell.StyleDefault)
		displayX += displayWidthRune(ch)
		runeIdx++
	}
}

func (e *Editor) drawLineWithHighlight(line string, startX, y int) {
	// Convert to runes for proper Unicode handling
	runes := []rune(line)

	// Apply horizontal scrolling as display-column based offset (not rune index)
	runeIdx, displayX := e.advanceToDisplayOffset(runes, y, startX, e.offsetX)

	if e.searchTerm == "" {
		e.drawPlainRun(runes, runeIdx, y, displayX)
		return
	}

	// Draw with search highlighting - Unicode-aware
	e.drawWithSearchHighlight(line, runes, runeIdx, y, displayX)
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
			runes := []rune(line)

			// Clamp coordinates to line bounds (rune-aware)
			if startX > len(runes) {
				startX = len(runes)
			}
			if endX > len(runes) {
				endX = len(runes)
			}

			// Apply selection highlight with proper Unicode positioning
			displayX := 0
			for runeIdx := 0; runeIdx < len(runes) && displayX < e.width; runeIdx++ {
				screenX := displayX - e.offsetX
				if runeIdx >= startX && runeIdx < endX && screenX >= 0 && screenX < e.width {
					ch := runes[runeIdx]
					e.screen.SetContent(screenX, screenY, ch, nil, selectionStyle)
				}
				displayX += displayWidthRune(runes[runeIdx])
			}
		}
	} else {
		// Multi-line selection
		for y := startY; y <= endY; y++ {
			screenY := y - e.offsetY
			if screenY >= 0 && screenY < e.height-1 && y < len(e.lines) {
				line := e.lines[y]
				runes := []rune(line)

				var lineStartX, lineEndX int
				if y == startY {
					lineStartX = startX
					lineEndX = len(runes)
				} else if y == endY {
					lineStartX = 0
					lineEndX = endX
				} else {
					lineStartX = 0
					lineEndX = len(runes)
				}

				// Clamp coordinates to line bounds
				if lineStartX > len(runes) {
					lineStartX = len(runes)
				}
				if lineEndX > len(runes) {
					lineEndX = len(runes)
				}

				// Apply selection highlight with proper Unicode positioning
				displayX := 0
				for runeIdx := 0; runeIdx < len(runes) && displayX < e.width; runeIdx++ {
					screenX := displayX - e.offsetX
					if runeIdx >= lineStartX && runeIdx < lineEndX && screenX >= 0 && screenX < e.width {
						ch := runes[runeIdx]
						e.screen.SetContent(screenX, screenY, ch, nil, selectionStyle)
					}
					displayX += displayWidthRune(runes[runeIdx])
				}
			}
		}
	}
}

// drawSelectionWrapped is removed - no longer needed for horizontal scrolling

func (e *Editor) draw() {
	e.screen.Clear()

	// Draw visible lines with horizontal scrolling
	screenRow := 0
	for lineIdx := e.offsetY; lineIdx < len(e.lines) && screenRow < e.height-1; lineIdx++ {
		line := e.lines[lineIdx]
		e.drawLineWithHighlight(line, 0, screenRow)
		screenRow++
	}

	// Draw selection
	e.drawSelection()

	// Draw status bar
	e.drawStatusBar()

	// Calculate cursor screen position with horizontal scrolling
	screenCursorY := e.cursorY - e.offsetY
	screenCursorX := 0

	// Calculate display width of text before cursor for proper positioning
	if e.cursorY < len(e.lines) {
		line := e.lines[e.cursorY]
		runes := []rune(line)

		// Calculate cursor position accounting for Unicode display widths
		for i := 0; i < e.cursorX && i < len(runes); i++ {
			screenCursorX += displayWidthRune(runes[i])
		}

		// Apply horizontal offset
		screenCursorX -= e.offsetX
	}

	// Show cursor if it's visible on screen
	if screenCursorY >= 0 && screenCursorY < e.height-1 &&
		screenCursorX >= 0 && screenCursorX < e.width {
		e.screen.ShowCursor(screenCursorX, screenCursorY)
	} else {
		// Hide cursor when it's off-screen
		e.screen.HideCursor()
	}

	e.screen.Show()
}

// ensureCursorVisible adjusts the viewport to keep the cursor visible
// Only call this when the cursor actually moves (keyboard, click, text editing)
// NOT during mouse wheel scrolling (which should be independent)
func (e *Editor) ensureCursorVisible() {
	// Vertical scrolling - ensure cursor line is visible
	if e.cursorY < e.offsetY {
		e.offsetY = e.cursorY
	}
	if e.cursorY >= e.offsetY+e.height-1 {
		e.offsetY = e.cursorY - (e.height - 2)
		if e.offsetY < 0 {
			e.offsetY = 0
		}
	}

	// Horizontal scrolling - ensure cursor is visible horizontally
	if e.cursorY < len(e.lines) {
		line := e.lines[e.cursorY]
		runes := []rune(line)

		// Calculate cursor display position
		cursorDisplayX := 0
		for i := 0; i < e.cursorX && i < len(runes); i++ {
			cursorDisplayX += displayWidthRune(runes[i])
		}

		// Adjust horizontal offset to keep cursor visible with a 5-column margin
		const margin = 5
		leftBound := e.offsetX + margin
		rightBound := e.offsetX + e.width - 1 - margin

		if cursorDisplayX < leftBound {
			e.offsetX = cursorDisplayX - margin
			if e.offsetX < 0 {
				e.offsetX = 0
			}
		}
		if cursorDisplayX > rightBound {
			e.offsetX = cursorDisplayX - (e.width - 1 - margin)
			if e.offsetX < 0 {
				e.offsetX = 0
			}
		}
	}
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
	col := x
	for _, r := range text {
		e.screen.SetContent(col, y, r, nil, style)
		col += displayWidthRune(r)
		if col >= e.width {
			break
		}
	}
}

func (e *Editor) prompt(prompt string) string {
	// Draw the prompt
	e.drawStatusBar()
	e.drawText(0, e.height-1, prompt, tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite))
	e.screen.Show()

	// Wait for user input (Unicode-aware accumulation)
	input := []rune("")
	for {
		ev := e.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEnter:
				return string(input)
			case tcell.KeyEscape:
				return ""
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
			default:
				if ev.Rune() != 0 {
					input = append(input, ev.Rune())
				}
			}
		}
		// Update the prompt with user input
		e.drawStatusBar()
		e.drawText(0, e.height-1, prompt+string(input), tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite))
		e.screen.Show()
	}
}

// promptFilename provides a simple filename prompt
func (e *Editor) promptFilename(title, initial string) string {
	e.drawStatusBar()
	input := []rune(initial)
	cursor := len(input)
	baseStyle := tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)

	redraw := func() {
		text := fmt.Sprintf("%s: %s", title, string(input))
		e.renderPromptLine(baseStyle, text, "")
	}

	redraw()

	for {
		ev := e.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEnter:
				return string(input)
			case tcell.KeyEscape:
				return ""
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if cursor > 0 {
					input = append(input[:cursor-1], input[cursor:]...)
					cursor--
				}
			default:
				if r := ev.Rune(); r != 0 {
					input = append(input[:cursor], append([]rune{r}, input[cursor:]...)...)
					cursor++
				}
			}
		}
		redraw()
	}
}

// promptYesNo asks a yes/no question and returns true for yes, false for no
func (e *Editor) promptYesNo(question string) bool {
	response := e.prompt(question + " (y/n): ")
	return response == "y" || response == "Y"
}

// Helper used by prompt rendering to place main text and optional right-side hint
func (e *Editor) renderPromptLine(style tcell.Style, text, extra string) {
	e.drawStatusBar()
	e.drawText(0, e.height-1, text, style)
	if extra != "" {
		startX := e.width - displayWidth(extra) - 1
		textWidth := displayWidth(text)
		if startX < textWidth+1 {
			startX = textWidth + 1
		}
		if startX < e.width {
			e.drawText(startX, e.height-1, extra, style)
		}
	}
	e.screen.Show()
}

