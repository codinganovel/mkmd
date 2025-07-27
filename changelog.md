# Changelog

All notable changes to mkmd will be documented in this file.

## [0.3] - 2025-01-25

### Added
- Save prompt when navigating between chunks with unsaved changes
- Enhanced help text explaining chunk behavior and save prompts
- Clear documentation of chunk "spillover" behavior

### Improved
- Consistent UX between chunk navigation and exit behavior
- No more accidental data loss when switching chunks
- Better user understanding of large file chunk mechanics

## [0.2] - 2025-01-25

### Added
- Large file chunk navigation with `Ctrl+T` (next chunk) and `Ctrl+B` (previous chunk)
- Enhanced status bar with navigation hints for chunked files
- Support for navigating through files larger than 10K lines
- Comprehensive test coverage for chunk functionality

### Changed
- Reduced chunk size from 100,000 lines to 10,000 lines for better performance
- Updated help text to include new chunk navigation shortcuts

### Fixed
- Critical bug where saving in chunk mode would erase parts of the file
- Chunk-aware saving now preserves entire file while only modifying the current chunk

### Improved
- Large files are now actually usable instead of just being truncated
- Better terminal compatibility by using simple Ctrl combinations instead of Shift modifiers
- Data integrity guaranteed when editing chunked files

## [0.1] - Initial Release

### Added
- Distraction-free terminal text editor
- Automatic indentation for markdown
- Visual text selection with highlighting
- Search functionality with highlighting
- Undo/redo support
- Mouse support
- Word-based navigation
- Standard editing shortcuts (Ctrl+X/C/V, etc.)
- Large file handling (initial truncation approach)