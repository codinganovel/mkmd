package main

import (
	"fmt"
	"log"
	"os"
)

// CLI entrypoint. Editor implementation is in other files.
func main() {
	args := os.Args[1:]
	var filename string
	switch len(args) {
	case 0:
		// Open an empty buffer (no filename yet)
		filename = ""
	case 1:
		filename = args[0]
	default:
		fmt.Fprintf(os.Stderr, "Usage: %s [filename]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nRun without an argument to open an empty buffer.\n")
		os.Exit(1)
	}

	editor, err := NewEditor(filename)
	if err != nil {
		log.Fatalf("Failed to create editor: %v", err)
	}

	if err := editor.run(); err != nil {
		log.Fatalf("Editor error: %v", err)
	}
}
