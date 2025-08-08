package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

func (e *Editor) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	buttons := ev.Buttons()

	// Handle scroll wheel/trackpad events first (they can occur with any button state)
	// Check for any wheel event flags using bitwise operations
	wheelEvent := false
	scrollAmount := 1 // Default scroll amount for smooth trackpad experience

	if buttons&tcell.WheelUp != 0 {
		wheelEvent = true
		// Add upward momentum (negative delta)
		e.addScrollMomentum(-float64(scrollAmount * 15)) // Multiply for more responsive feel
	} else if buttons&tcell.WheelDown != 0 {
		wheelEvent = true
		// Add downward momentum (positive delta)
		e.addScrollMomentum(float64(scrollAmount * 15)) // Multiply for more responsive feel
	} else if buttons&tcell.WheelLeft != 0 {
		// Horizontal scroll left (trackpad gesture)
		wheelEvent = true
		e.offsetX -= 3 // Scroll left by 3 characters
		if e.offsetX < 0 {
			e.offsetX = 0
		}
	} else if buttons&tcell.WheelRight != 0 {
		// Horizontal scroll right (trackpad gesture)
		wheelEvent = true
		e.offsetX += 3 // Scroll right by 3 characters
	}

	// If we handled a wheel event, return early
	if wheelEvent {
		return
	}

	// Handle regular mouse button events (clicks, drags, etc.)
	switch buttons {
	case tcell.Button1: // Left click
		// Convert screen coordinates to line/column with horizontal scrolling
		screenRow := y
		screenCol := x

		// Validate coordinates and don't allow clicking on status bar
		if screenRow >= 0 && screenRow < e.height-1 {
			// Calculate target line accounting for vertical scroll
			targetLineY := screenRow + e.offsetY
			if targetLineY >= 0 && targetLineY < len(e.lines) {
				e.cursorY = targetLineY

				// Calculate target column accounting for horizontal scroll and Unicode
				line := e.lines[targetLineY]
				runes := []rune(line)

				// Find the rune position that corresponds to the clicked screen position
				targetDisplayX := screenCol + e.offsetX
				currentDisplayX := 0
				targetRuneX := 0

				for i, r := range runes {
					runeWidth := displayWidthRune(r)
					if currentDisplayX+runeWidth/2 > targetDisplayX {
						// Click is closer to this rune position
						break
					}
					currentDisplayX += runeWidth
					targetRuneX = i + 1
				}

				// Clamp to valid range
				if targetRuneX > len(runes) {
					targetRuneX = len(runes)
				}

				e.cursorX = targetRuneX
				e.clearSelection()
				e.ensureCursorVisible()
			}
		}
	case tcell.ButtonNone:
		// Handle case where mouse moves without buttons pressed
		// This can include some wheel events on certain terminals
		// Most wheel events should be caught above, but this provides fallback
		break
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
				if err := e.saveFileWithPrompt(); err != nil {
					return fmt.Errorf("failed to save file: %v", err)
				}
				return nil

			case tcell.KeyCtrlS:
				// Save file
				if err := e.saveFileWithPrompt(); err != nil {
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
					e.cursorX = runeLen(e.lines[e.cursorY])
				}

			case tcell.KeyCtrlF:
				// Classic prompt search
				e.search()

			case tcell.KeyF4:
				// Incremental search
				e.searchIncremental()

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
				// Copy
				if e.selectionStart {
					e.copy()
				}

			case tcell.KeyCtrlQ:
				// Quit
				if e.modified {
					response := e.prompt("Save changes? (y/n): ")
					if response == "y" {
						if err := e.saveFileWithPrompt(); err != nil {
							return fmt.Errorf("failed to save file: %v", err)
						}
					}
				}
				return nil

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
					e.ensureCursorVisible()
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
						e.cursorX = runeLen(e.lines[e.cursorY])
					}
					e.ensureCursorVisible()
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
					e.ensureCursorVisible()
				} else {
					// Check if Shift is pressed for selection
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					if e.cursorY < len(e.lines) && e.cursorX < runeLen(e.lines[e.cursorY]) {
						e.cursorX++
					} else if e.cursorY < len(e.lines)-1 {
						e.cursorY++
						e.cursorX = 0
					}
					e.ensureCursorVisible()
				}

			case tcell.KeyHome:
				// Check if Ctrl is pressed for document start
				if ev.Modifiers()&tcell.ModCtrl != 0 {
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					// Go to beginning of document
					e.cursorY = 0
					e.cursorX = 0
					e.ensureCursorVisible()
				} else {
					// Regular Home - go to beginning of line
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					e.cursorX = 0
					e.ensureCursorVisible()
				}

			case tcell.KeyEnd:
				// Check if Ctrl is pressed for document end
				if ev.Modifiers()&tcell.ModCtrl != 0 {
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					// Go to end of document
					e.cursorY = len(e.lines) - 1
					if e.cursorY >= 0 && e.cursorY < len(e.lines) {
						e.cursorX = runeLen(e.lines[e.cursorY])
					}
					e.ensureCursorVisible()
				} else {
					// Regular End - go to end of line
					if ev.Modifiers()&tcell.ModShift != 0 {
						e.startSelection()
					} else {
						e.clearSelection()
					}
					if e.cursorY < len(e.lines) {
						e.cursorX = runeLen(e.lines[e.cursorY])
					}
					e.ensureCursorVisible()
				}

			case tcell.KeyPgUp:
				e.clearSelection()
				e.cursorY -= e.height - 1
				if e.cursorY < 0 {
					e.cursorY = 0
				}
				e.ensureCursorVisible()

			case tcell.KeyPgDn:
				e.clearSelection()
				e.cursorY += e.height - 1
				if e.cursorY >= len(e.lines) {
					e.cursorY = len(e.lines) - 1
				}
				e.ensureCursorVisible()

			case tcell.KeyUp:
				// Check if Shift is pressed for selection
				if ev.Modifiers()&tcell.ModShift != 0 {
					e.startSelection()
				} else {
					e.clearSelection()
				}
				if e.cursorY > 0 {
					e.cursorY--
					if e.cursorX > runeLen(e.lines[e.cursorY]) {
						e.cursorX = runeLen(e.lines[e.cursorY])
					}
				}
				e.ensureCursorVisible()

			case tcell.KeyDown:
				// Check if Shift is pressed for selection
				if ev.Modifiers()&tcell.ModShift != 0 {
					e.startSelection()
				} else {
					e.clearSelection()
				}
				if e.cursorY < len(e.lines)-1 {
					e.cursorY++
					if e.cursorX > runeLen(e.lines[e.cursorY]) {
						e.cursorX = runeLen(e.lines[e.cursorY])
					}
				}
				e.ensureCursorVisible()

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
		e.applyScrollMomentum() // Apply momentum scrolling with decay
		e.draw()
	}
}
