package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Cleanup handles post-removal steps: ent regen, go mod tidy, build verification, and self-removal.
type Cleanup struct {
	root    string
	verbose bool
}

// NewCleanup creates a Cleanup rooted at the given project directory.
func NewCleanup(root string, verbose bool) *Cleanup {
	return &Cleanup{root: root, verbose: verbose}
}

// CleanStaleEntFiles deletes all generated Ent files/dirs, preserving only
// the schema definitions and codegen configuration.
func (c *Cleanup) CleanStaleEntFiles() error {
	entDir := filepath.Join(c.root, "ent")

	// Preserved files in ent/ root.
	preserved := map[string]bool{
		"schema":      true, // directory
		"generate.go": true,
		"entc.go":     true,
	}

	entries, err := os.ReadDir(entDir)
	if err != nil {
		return fmt.Errorf("failed to read ent directory: %w", err)
	}

	for _, entry := range entries {
		if preserved[entry.Name()] {
			continue
		}

		p := filepath.Join(entDir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(p); err != nil {
				return fmt.Errorf("failed to remove ent dir %s: %w", entry.Name(), err)
			}
		} else {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("failed to remove ent file %s: %w", entry.Name(), err)
			}
		}

		if c.verbose {
			fmt.Printf("  cleaned: ent/%s\n", entry.Name())
		}
	}

	return nil
}

// RegenerateEnt runs `go generate ./ent` to rebuild ORM code from remaining schemas.
func (c *Cleanup) RegenerateEnt() error {
	return c.runCmd("go", "generate", "./ent")
}

// GoModTidy runs `go mod tidy` to remove unused Go dependencies.
func (c *Cleanup) GoModTidy() error {
	return c.runCmd("go", "mod", "tidy")
}

// VerifyGoBuild runs `go build -o /dev/null ./cmd/web` as a hard gate.
func (c *Cleanup) VerifyGoBuild() error {
	return c.runCmd("go", "build", "-o", "/dev/null", "./cmd/web")
}

// VerifyFrontendBuild runs `npx tsc --noEmit` as a hard gate.
// Always warns visibly when skipping due to missing node_modules.
func (c *Cleanup) VerifyFrontendBuild() error {
	nodeModules := filepath.Join(c.root, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		fmt.Println("  ⚠ skipped (node_modules not found)")
		fmt.Println("    Run `npm install && npx tsc --noEmit` to verify frontend build manually.")
		return nil
	}
	return c.runCmd("npx", "tsc", "--noEmit")
}

// SelfCleanup removes the setup tool itself and cleans up.
func (c *Cleanup) SelfCleanup() error {
	// Remove cmd/setup/ directory.
	setupDir := filepath.Join(c.root, "cmd", "setup")
	if err := os.RemoveAll(setupDir); err != nil {
		return fmt.Errorf("failed to remove cmd/setup: %w", err)
	}

	if c.verbose {
		fmt.Println("  removed: cmd/setup/")
	}

	// Remove the huh dependency and run go mod tidy again.
	if err := c.runCmd("go", "get", "-u", "github.com/charmbracelet/huh@none"); err != nil {
		// Ignore error — go mod tidy will clean it up anyway.
		if c.verbose {
			fmt.Printf("  note: could not remove huh dependency directly: %v\n", err)
		}
	}

	if err := c.GoModTidy(); err != nil {
		return fmt.Errorf("final go mod tidy failed: %w", err)
	}

	// Remove the make setup target from Makefile.
	c.removeMakeSetupTarget()

	return nil
}

// removeMakeSetupTarget removes the `setup` target from the Makefile.
func (c *Cleanup) removeMakeSetupTarget() {
	makefile := filepath.Join(c.root, "Makefile")
	data, err := os.ReadFile(makefile)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	skip := false

	for _, line := range lines {
		if strings.HasPrefix(line, ".PHONY: setup") || strings.HasPrefix(line, "setup:") {
			skip = true
			continue
		}
		if skip {
			// Skip continuation lines (lines starting with tab).
			if strings.HasPrefix(line, "\t") {
				continue
			}
			// Skip blank lines immediately after the target.
			if strings.TrimSpace(line) == "" {
				skip = false
				continue
			}
			skip = false
		}
		result = append(result, line)
	}

	_ = os.WriteFile(makefile, []byte(strings.Join(result, "\n")), 0644)
}

// runCmd executes a command in the project root and returns any error.
func (c *Cleanup) runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = c.root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if c.verbose {
		fmt.Printf("  running: %s %s\n", name, strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
