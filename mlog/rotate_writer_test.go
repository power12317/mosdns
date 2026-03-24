package mlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyRotateWriteSyncer_RotateAndPrune(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mosdns.log")

	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.Local)
	w, err := newDailyRotateWriteSyncer(logPath, 2, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("day1\n")); err != nil {
		t.Fatal(err)
	}

	oldRotated := logPath + ".2026-03-21"
	if err := os.WriteFile(oldRotated, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now = now.AddDate(0, 0, 1)
	if _, err := w.Write([]byte("day2\n")); err != nil {
		t.Fatal(err)
	}

	gotCurrent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotCurrent) != "day2\n" {
		t.Fatalf("current log = %q, want %q", gotCurrent, "day2\n")
	}

	gotRotated, err := os.ReadFile(logPath + ".2026-03-24")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRotated) != "day1\n" {
		t.Fatalf("rotated log = %q, want %q", gotRotated, "day1\n")
	}

	if _, err := os.Stat(oldRotated); !os.IsNotExist(err) {
		t.Fatalf("old rotated file still exists, err=%v", err)
	}
}

func TestDailyRotateWriteSyncer_RotateStaleFileOnOpen(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mosdns.log")

	if err := os.WriteFile(logPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleDay := time.Date(2026, 3, 20, 23, 0, 0, 0, time.Local)
	if err := os.Chtimes(logPath, staleDay, staleDay); err != nil {
		t.Fatal(err)
	}
	existingRotated := logPath + ".2026-03-20"
	if err := os.WriteFile(existingRotated, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.Local)
	w, err := newDailyRotateWriteSyncer(logPath, 7, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("today\n")); err != nil {
		t.Fatal(err)
	}

	gotRotated, err := os.ReadFile(existingRotated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotRotated), "existing\nstale\n") {
		t.Fatalf("unexpected rotated content: %q", gotRotated)
	}

	gotCurrent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotCurrent) != "today\n" {
		t.Fatalf("current log = %q, want %q", gotCurrent, "today\n")
	}
}
