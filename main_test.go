package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeDiffShowsChangedLines(t *testing.T) {
	fm := &FileMonitor{}

	diff := fm.computeDiff("before\nsame\n", "after\nsame\n", "test.txt")

	for _, want := range []string{"-before\n", "+after\n", " same\n"} {
		if !strings.Contains(diff, want) {
			t.Errorf("diff %q does not contain %q", diff, want)
		}
	}
}

func TestProcessCreateReportsReplacementDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watched.txt")
	if err := os.WriteFile(path, []byte("new content\n"), 0600); err != nil {
		t.Fatal(err)
	}

	fm := &FileMonitor{
		files: map[string]*FileContent{
			path: {content: "old content\n"},
		},
	}

	output := captureStdout(t, func() {
		fm.processCreate(path)
	})

	for _, want := range []string{"File modified:", "-old content\n", "+new content\n"} {
		if !strings.Contains(output, want) {
			t.Errorf("output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "Size:") {
		t.Errorf("replacement was reported as a size-only change: %q", output)
	}
	if got := fm.files[path].content; got != "new content\n" {
		t.Errorf("tracked content = %q, want %q", got, "new content\n")
	}
}

func TestProcessRemoveThenCreateReportsDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watched.txt")
	if err := os.WriteFile(path, []byte("new content\n"), 0600); err != nil {
		t.Fatal(err)
	}

	fm := &FileMonitor{
		files: map[string]*FileContent{
			path: {content: "old content\n"},
		},
	}

	output := captureStdout(t, func() {
		fm.processRemove(path)
		fm.processCreate(path)
	})

	for _, want := range []string{"File modified:", "-old content\n", "+new content\n"} {
		if !strings.Contains(output, want) {
			t.Errorf("output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "File removed:") {
		t.Errorf("replacement was incorrectly reported as removed: %q", output)
	}
}

func TestOpenLogFileCreatesDirectoryAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "logs", "watch.log")

	file, err := openLogFile(path)
	if err != nil {
		t.Fatalf("openLogFile returned an error: %v", err)
	}
	logWriter := &synchronizedWriter{writer: file}

	var console bytes.Buffer
	output := io.MultiWriter(&console, logWriter)
	if _, err := io.WriteString(output, "monitor started\n"); err != nil {
		t.Fatalf("write output: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close log file: %v", err)
	}

	file, err = openLogFile(path)
	if err != nil {
		t.Fatalf("reopen log file: %v", err)
	}
	defer file.Close()
	if _, err := io.WriteString(&synchronizedWriter{writer: file}, "file changed\n"); err != nil {
		t.Fatalf("append to log file: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if got, want := string(content), "monitor started\nfile changed\n"; got != want {
		t.Errorf("log content = %q, want %q", got, want)
	}
	if got, want := console.String(), "monitor started\n"; got != want {
		t.Errorf("console output = %q, want %q", got, want)
	}
}

func TestOpenLogFileRequiresPath(t *testing.T) {
	if _, err := openLogFile("  "); err == nil {
		t.Fatal("openLogFile accepted an empty path")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdout

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
