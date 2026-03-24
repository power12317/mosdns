package mlog

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

const rotateDateLayout = "2006-01-02"

var timeNow = time.Now

type dailyRotateWriteSyncer struct {
	mu       sync.Mutex
	path     string
	dir      string
	baseName string
	keepDays int
	now      func() time.Time

	file *os.File
	day  string
}

var _ zapcore.WriteSyncer = (*dailyRotateWriteSyncer)(nil)

func newDailyRotateWriteSyncer(path string, keepDays int, now func() time.Time) (*dailyRotateWriteSyncer, error) {
	if now == nil {
		now = time.Now
	}
	w := &dailyRotateWriteSyncer{
		path:     path,
		dir:      filepath.Dir(path),
		baseName: filepath.Base(path),
		keepDays: keepDays,
		now:      now,
	}
	if err := w.openLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyRotateWriteSyncer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeededLocked(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *dailyRotateWriteSyncer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *dailyRotateWriteSyncer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeLocked()
}

func (w *dailyRotateWriteSyncer) openLocked() error {
	now := w.now()
	if err := w.rotateStaleActiveLocked(now); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.day = now.Format(rotateDateLayout)
	return w.pruneLocked(now)
}

func (w *dailyRotateWriteSyncer) rotateIfNeededLocked() error {
	currentDay := w.now().Format(rotateDateLayout)
	if w.file == nil {
		return w.openLocked()
	}
	if w.day == currentDay {
		return nil
	}
	previousDay := w.day
	if err := w.closeLocked(); err != nil {
		return err
	}
	if err := archiveFile(w.path, w.rotatedPath(previousDay)); err != nil {
		return err
	}
	return w.openLocked()
}

func (w *dailyRotateWriteSyncer) rotateStaleActiveLocked(now time.Time) error {
	st, err := os.Stat(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	fileDay := st.ModTime().Format(rotateDateLayout)
	currentDay := now.Format(rotateDateLayout)
	if fileDay == currentDay {
		return nil
	}
	return archiveFile(w.path, w.rotatedPath(fileDay))
}

func (w *dailyRotateWriteSyncer) pruneLocked(now time.Time) error {
	if w.keepDays <= 0 {
		return nil
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	cutoff := now.AddDate(0, 0, -w.keepDays).Format(rotateDateLayout)
	prefix := w.baseName + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		day := strings.TrimPrefix(name, prefix)
		if _, err := time.Parse(rotateDateLayout, day); err != nil {
			continue
		}
		if day < cutoff {
			if err := os.Remove(filepath.Join(w.dir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func (w *dailyRotateWriteSyncer) closeLocked() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *dailyRotateWriteSyncer) rotatedPath(day string) string {
	return w.path + "." + day
}

func archiveFile(src, dst string) error {
	if src == dst {
		return nil
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		if err := appendFile(src, dst); err != nil {
			return err
		}
		return os.Remove(src)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}

func appendFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
