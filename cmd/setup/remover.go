package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Remover handles file deletion and marker-based patching.
type Remover struct {
	root    string
	verbose bool
}

// NewRemover creates a Remover rooted at the given project directory.
func NewRemover(root string, verbose bool) *Remover {
	return &Remover{root: root, verbose: verbose}
}

// DeleteFiles removes standalone files listed in the module manifest.
// Missing files are silently skipped.
func (r *Remover) DeleteFiles(files []string) (int, error) {
	count := 0
	for _, f := range files {
		p := filepath.Join(r.root, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if r.verbose {
				fmt.Printf("  skip (not found): %s\n", f)
			}
			continue
		}
		if err := os.Remove(p); err != nil {
			return count, fmt.Errorf("failed to delete %s: %w", f, err)
		}
		count++
		if r.verbose {
			fmt.Printf("  deleted: %s\n", f)
		}
	}
	return count, nil
}

// DeleteDirs removes directories recursively.
// Missing directories are silently skipped.
func (r *Remover) DeleteDirs(dirs []string) (int, error) {
	count := 0
	for _, d := range dirs {
		p := filepath.Join(r.root, d)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if r.verbose {
				fmt.Printf("  skip dir (not found): %s\n", d)
			}
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return count, fmt.Errorf("failed to delete dir %s: %w", d, err)
		}
		count++
		if r.verbose {
			fmt.Printf("  deleted dir: %s\n", d)
		}
	}
	return count, nil
}

// PatchFeatureMarkers removes all [feature:name] start/end blocks from all
// files in the project that contain them.
func (r *Remover) PatchFeatureMarkers(featureNames []string) (int, error) {
	filesPatched := 0

	for _, name := range featureNames {
		files, err := r.findFilesWithMarker(name)
		if err != nil {
			return filesPatched, err
		}

		for _, f := range files {
			patched, err := r.removeMarkerBlocks(f, name)
			if err != nil {
				return filesPatched, fmt.Errorf("failed to patch %s for feature %s: %w", f, name, err)
			}
			if patched {
				filesPatched++
				if r.verbose {
					fmt.Printf("  patched: %s (removed [feature:%s] blocks)\n", relPath(r.root, f), name)
				}
			}
		}
	}

	return filesPatched, nil
}

// findFilesWithMarker walks the project looking for files containing the given
// feature marker. It skips node_modules, .git, and vendor directories.
func (r *Remover) findFilesWithMarker(featureName string) ([]string, error) {
	marker := fmt.Sprintf("[feature:%s]", featureName)
	var matches []string

	err := filepath.Walk(r.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories we don't care about.
		if info.IsDir() {
			base := info.Name()
			if base == "node_modules" || base == ".git" || base == "vendor" {
				return filepath.SkipDir
			}
			if base == "setup" && filepath.Dir(path) == filepath.Join(r.root, "cmd") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process text-like files.
		ext := filepath.Ext(path)
		switch ext {
		case ".go", ".tsx", ".ts", ".yaml", ".yml", ".jsx", ".js":
		default:
			// Also check Makefile by name.
			if info.Name() == "Makefile" {
				// fall through
			} else {
				return nil
			}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), marker) {
			matches = append(matches, path)
		}
		return nil
	})

	return matches, err
}

// removeMarkerBlocks removes all [feature:name] start/end delimited blocks
// from the given file. Returns true if the file was modified.
func (r *Remover) removeMarkerBlocks(filePath, featureName string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	original := string(data)
	result := original

	// Build patterns for different comment styles.
	// Go/Makefile: // [feature:X] start or # [feature:X] start
	// TSX/JS: // [feature:X] start or {/* [feature:X] start */}
	// YAML: # [feature:X] start
	patterns := []string{
		// Go / JS / TS single-line comments
		fmt.Sprintf(`(?m)^[^\S\n]*//\s*\[feature:%s\]\s*start\s*\n`, regexp.QuoteMeta(featureName)),
		// YAML / Makefile hash comments
		fmt.Sprintf(`(?m)^[^\S\n]*#\s*\[feature:%s\]\s*start\s*\n`, regexp.QuoteMeta(featureName)),
		// JSX comments
		fmt.Sprintf(`(?m)^[^\S\n]*\{/\*\s*\[feature:%s\]\s*start\s*\*/\}\s*\n`, regexp.QuoteMeta(featureName)),
	}

	endPatterns := []string{
		fmt.Sprintf(`(?m)^[^\S\n]*//\s*\[feature:%s\]\s*end\s*\n?`, regexp.QuoteMeta(featureName)),
		fmt.Sprintf(`(?m)^[^\S\n]*#\s*\[feature:%s\]\s*end\s*\n?`, regexp.QuoteMeta(featureName)),
		fmt.Sprintf(`(?m)^[^\S\n]*\{/\*\s*\[feature:%s\]\s*end\s*\*/\}\s*\n?`, regexp.QuoteMeta(featureName)),
	}

	for i, startPat := range patterns {
		endPat := endPatterns[i]

		startRe := regexp.MustCompile(startPat)
		endRe := regexp.MustCompile(endPat)

		for {
			startLoc := startRe.FindStringIndex(result)
			if startLoc == nil {
				break
			}

			// Find matching end after this start.
			remainder := result[startLoc[1]:]
			endLoc := endRe.FindStringIndex(remainder)
			if endLoc == nil {
				// No matching end marker — warn but don't fail.
				fmt.Printf("  warning: no matching end marker for [feature:%s] in %s\n", featureName, relPath(r.root, filePath))
				break
			}

			// Remove from start of start-marker to end of end-marker.
			result = result[:startLoc[0]] + result[startLoc[1]+endLoc[1]:]
		}
	}

	if result == original {
		return false, nil
	}

	// Post-cleanup.
	result = cleanupArtifacts(result, filePath)

	info, _ := os.Stat(filePath)
	if err := os.WriteFile(filePath, []byte(result), info.Mode()); err != nil {
		return false, err
	}
	return true, nil
}

// cleanupArtifacts handles post-removal cleanup:
// - Collapse consecutive blank lines to one
// - Remove empty import groups: import ()
// - Remove dangling commas before closing braces in imports
func cleanupArtifacts(content, filePath string) string {
	// Collapse 3+ consecutive newlines to 2.
	multiBlank := regexp.MustCompile(`\n{3,}`)
	content = multiBlank.ReplaceAllString(content, "\n\n")

	ext := filepath.Ext(filePath)

	// Go-specific cleanup.
	if ext == ".go" {
		// Remove empty import blocks.
		emptyImport := regexp.MustCompile(`(?m)^import\s*\(\s*\)\s*\n?`)
		content = emptyImport.ReplaceAllString(content, "")
	}

	// TSX/JS-specific cleanup: remove trailing comma before closing } in imports.
	if ext == ".tsx" || ext == ".ts" || ext == ".jsx" || ext == ".js" {
		danglingComma := regexp.MustCompile(`,\s*\n(\s*}\s*from\s)`)
		content = danglingComma.ReplaceAllString(content, "\n$1")
	}

	return content
}

func relPath(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return rel
}
