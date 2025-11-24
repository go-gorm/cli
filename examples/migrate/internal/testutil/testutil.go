package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func RunMigrateTests(t *testing.T, projectDir string, args ...string) {
	t.Helper()

	runMigrateInit(t, projectDir)

	runMigrateReflect(t, projectDir)

	runMigrateDiff(t, projectDir)
}

// diffFiles runs a recursive unified diff between the expected and actual directories.
func diffFiles(t *testing.T, expected, actual string) {
	t.Helper()
	cmd := exec.Command("diff", "-u", "-r", "--new-file", expected, actual)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("directory mismatch:\n%s", buf.String())
	}
}

func runMigrateInit(t *testing.T, projectDir string) {
	t.Helper()
	if out, err := execMigrate(t, projectDir, "init", "--force"); err != nil {
		t.Fatalf("migrate init failed: %v\n%s", err, out)
	}
	expected := filepath.Join(projectDir, "outputs", "migrations", "main.go")
	actual := filepath.Join(projectDir, "migrations", "main.go")
	diffFiles(t, expected, actual)
}

func runMigrateReflect(t *testing.T, projectDir string) {
	t.Helper()
	if out, err := execMigrate(t, projectDir, "reflect"); err != nil {
		t.Fatalf("migrate reflect failed: %v\n%s", err, out)
	}
	expected := filepath.Join(projectDir, "outputs", "models")
	actual := filepath.Join(projectDir, "models")
	diffFiles(t, expected, actual)
}

func runMigrateDiff(t *testing.T, projectDir string) {
	t.Helper()
	out, err := execMigrate(t, projectDir, "diff")
	if err != nil {
		t.Fatalf("migrate diff failed: %v\n%s", err, out)
	}
	expected := filepath.Join(projectDir, "outputs", "diff", "matching.txt")
	assertEqualFile(t, expected, out)
}

func assertEqualFile(t *testing.T, expectedPath, actual string) {
	t.Helper()
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read expected output %s: %v", expectedPath, err)
	}
	if strings.TrimRight(actual, "\n") != strings.TrimRight(string(data), "\n") {
		t.Fatalf("unexpected diff output:\nexpected:\n%s\nactual:\n%s", string(data), actual)
	}
}

func execMigrate(t *testing.T, projectDir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "../../..", "migrate"}, args...)...)
	cmd.Dir = projectDir
	cmd.Stdin = strings.NewReader(strings.Repeat("y\n", 8))
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "gocache"))
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
