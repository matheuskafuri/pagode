package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
)

func main() {
	root, err := projectRoot()
	if err != nil {
		fatal("Failed to find project root: %v", err)
	}

	// Precondition: clean git working tree.
	if !isGitClean(root) {
		fatal("Please commit or stash changes before running setup.")
	}

	modules := Modules()

	// TUI: multi-select checklist. All features are pre-checked.
	// The user unchecks features they want to REMOVE.
	keepIndices := make([]int, len(modules))
	options := make([]huh.Option[int], len(modules))
	for i, m := range modules {
		keepIndices[i] = i
		options[i] = huh.NewOption(fmt.Sprintf("%s — %s", m.Name, m.Description), i).Selected(true)
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title("Pagode Setup").
				Description("Select the features to INCLUDE in your project.\nUncheck any features you don't need.").
				Options(options...).
				Value(&keepIndices),
		),
	).Run()

	if err != nil {
		fatal("Setup cancelled: %v", err)
	}

	// Determine which modules to remove.
	keepSet := make(map[int]bool)
	for _, i := range keepIndices {
		keepSet[i] = true
	}

	var toRemove []Module
	for i, m := range modules {
		if !keepSet[i] {
			toRemove = append(toRemove, m)
		}
	}

	if len(toRemove) == 0 {
		fmt.Println("\nNo changes needed.")
		return
	}

	// Display what will be removed.
	names := make([]string, len(toRemove))
	for i, m := range toRemove {
		names[i] = m.Name
	}
	fmt.Printf("\nRemoving: %s\n", strings.Join(names, ", "))

	remover := NewRemover(root, true)
	cleanup := NewCleanup(root, true)

	// Collect all feature names for marker patching.
	featureNames := featureNamesFromModules(toRemove)

	// Track if any ent schemas are removed.
	hasEntSchemas := false

	// Step 1: Delete feature files.
	step := 1
	totalSteps := 8
	fmt.Printf("[%d/%d] Deleting feature files...\n", step, totalSteps)
	totalFiles := 0
	totalDirs := 0
	for _, m := range toRemove {
		// Delete ent schemas first (they're files too).
		n, err := remover.DeleteFiles(m.EntSchemas)
		if err != nil {
			fatal("Error deleting ent schemas: %v", err)
		}
		totalFiles += n
		if len(m.EntSchemas) > 0 {
			hasEntSchemas = true
		}

		n, err = remover.DeleteFiles(m.Files)
		if err != nil {
			fatal("Error deleting files: %v", err)
		}
		totalFiles += n

		dn, err := remover.DeleteDirs(m.Dirs)
		if err != nil {
			fatal("Error deleting directories: %v", err)
		}
		totalDirs += dn
	}
	fmt.Printf("         done (%d files, %d directories)\n", totalFiles, totalDirs)

	// Step 2: Patch shared files (remove marker blocks).
	step++
	fmt.Printf("[%d/%d] Patching shared files...\n", step, totalSteps)
	patchCount, err := remover.PatchFeatureMarkers(featureNames)
	if err != nil {
		fatal("Error patching shared files: %v", err)
	}
	fmt.Printf("         done (%d files patched)\n", patchCount)

	// Step 3: Clean stale Ent generated files.
	step++
	fmt.Printf("[%d/%d] Cleaning stale Ent files...\n", step, totalSteps)
	if hasEntSchemas {
		if err := cleanup.CleanStaleEntFiles(); err != nil {
			fatal("Error cleaning Ent files: %v", err)
		}
	}
	fmt.Println("         done")

	// Step 4: Regenerate Ent ORM.
	step++
	fmt.Printf("[%d/%d] Regenerating Ent ORM...\n", step, totalSteps)
	if hasEntSchemas {
		if err := cleanup.RegenerateEnt(); err != nil {
			fatal("Error regenerating Ent: %v", err)
		}
	}
	fmt.Println("         done")

	// Step 5: Clean Go dependencies.
	step++
	fmt.Printf("[%d/%d] Cleaning Go dependencies...\n", step, totalSteps)
	if err := cleanup.GoModTidy(); err != nil {
		fatal("Error running go mod tidy: %v", err)
	}
	fmt.Println("         done")

	// Step 6: Verify Go build (HARD GATE).
	step++
	fmt.Printf("[%d/%d] Verifying Go build...\n", step, totalSteps)
	if err := cleanup.VerifyGoBuild(); err != nil {
		fmt.Println("\n❌ Go build failed! The setup tool has NOT been cleaned up.")
		fmt.Println("   Please fix the build errors and run setup again, or use `git checkout .` to recover.")
		os.Exit(1)
	}
	fmt.Println("         done")

	// Step 7: Verify frontend build (HARD GATE).
	step++
	fmt.Printf("[%d/%d] Verifying frontend build...\n", step, totalSteps)
	if err := cleanup.VerifyFrontendBuild(); err != nil {
		fmt.Println("\n❌ Frontend build failed! The setup tool has NOT been cleaned up.")
		fmt.Println("   Please fix the TypeScript errors and run setup again, or use `git checkout .` to recover.")
		os.Exit(1)
	}
	fmt.Println("         done")

	// Step 8: Self-cleanup.
	step++
	fmt.Printf("[%d/%d] Cleaning up setup tool...\n", step, totalSteps)
	if err := cleanup.SelfCleanup(); err != nil {
		fatal("Error during self-cleanup: %v", err)
	}
	fmt.Println("         done")

	fmt.Println("\n✅ Setup complete! Run `make run` to start your project.")
}

// featureNamesFromModules extracts the short feature name used in markers
// from the module Name field.
func featureNamesFromModules(modules []Module) []string {
	nameMap := map[string]string{
		"Payment (Stripe)": "payment",
		"Chat (WebSocket)":  "chat",
		"Mail (Resend)":     "mail",
		"Background Tasks":  "tasks",
		"File Upload":       "files",
	}

	var names []string
	seen := make(map[string]bool)
	for _, m := range modules {
		if n, ok := nameMap[m.Name]; ok && !seen[n] {
			names = append(names, n)
			seen[n] = true
		}
	}
	return names
}

// projectRoot finds the project root by looking for go.mod.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir, nil
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// isGitClean checks if the git working tree is clean.
func isGitClean(root string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// Filter out untracked files in cmd/setup/ and docs/plans/ (expected).
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Allow untracked setup tool files and plan docs.
		if strings.Contains(line, "cmd/setup/") || strings.Contains(line, "docs/plans/") {
			continue
		}
		return false
	}
	return true
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
