package skillregistry

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const WatchDebounceMS = 500 * time.Millisecond
const PollInterval = 30 * time.Second

type Watcher struct {
	watcher *fsnotify.Watcher
	timer   *time.Timer
	ticker  *time.Ticker
	mu      sync.Mutex
	active  map[string]struct{}
	once    sync.Once
	root    string
	lastFP  string
}

func isSkillMD(n string) bool { return filepath.Base(n) == "SKILL.md" }
func shouldSkipWatcher() bool {
	if os.Getenv("BIGGZ_NO_SKILL_REGISTRY") == "1" || os.Getenv("GENTLE_PI_NO_SKILL_REGISTRY") == "1" {
		return true
	}
	for _, a := range os.Args {
		if a == "--no-skills" || a == "-ns" {
			return true
		}
	}
	return false
}
func watchRecursive(w *fsnotify.Watcher, root string, active map[string]struct{}) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Debug("walk err", "path", p, "err", err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if _, ok := active[p]; ok {
			return nil
		}
		if e := w.Add(p); e != nil {
			slog.Debug("add failed", "path", p, "err", e)
			return nil
		}
		active[p] = struct{}{}
		return nil
	})
}
func Start(projectRoot string, ctx context.Context) (*Watcher, error) {
	if shouldSkipWatcher() {
		slog.Debug("watcher gated")
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lastFP := Fingerprint(projectRoot)
	dirs := uniqueExistingDirs(projectRoot)
	fsW, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Debug("NewWatcher failed", "err", err)
		return startPolling(projectRoot, lastFP, ctx)
	}
	active := make(map[string]struct{})
	for _, d := range dirs {
		_ = watchRecursive(fsW, d, active)
	}
	if len(active) == 0 {
		_ = fsW.Close()
		slog.Debug("no active watches")
		return startPolling(projectRoot, lastFP, ctx)
	}
	tm := time.NewTimer(WatchDebounceMS)
	if !tm.Stop() {
		select { case <-tm.C: default: }
	}
	w := &Watcher{watcher: fsW, timer: tm, active: active, root: projectRoot, lastFP: lastFP}
	setGlobalWatcher(w)
	go w.loop(ctx)
	return w, nil
}
func startPolling(root, lastFP string, ctx context.Context) (*Watcher, error) {
	tk := time.NewTicker(PollInterval)
	w := &Watcher{ticker: tk, active: make(map[string]struct{}), root: root, lastFP: lastFP}
	setGlobalWatcher(w)
	go w.pollLoop(ctx)
	return w, nil
}
func (w *Watcher) loop(ctx context.Context) {
	defer w.Close()
	var tc <-chan time.Time
	if w.timer != nil {
		tc = w.timer.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					w.mu.Lock()
					_ = watchRecursive(w.watcher, ev.Name, w.active)
					w.mu.Unlock()
				}
			}
			if !isSkillMD(ev.Name) {
				continue
			}
			w.mu.Lock()
			if w.timer != nil {
				if !w.timer.Stop() {
					select { case <-w.timer.C: default: }
				}
				w.timer.Reset(WatchDebounceMS)
			}
			w.mu.Unlock()
		case e, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Debug("watcher err", "err", e)
		case <-tc:
			fp := Fingerprint(w.root)
			w.mu.Lock()
			last := w.lastFP
			w.mu.Unlock()
			if fp == last {
				continue
			}
			if r, err := Refresh(w.root, false); err != nil {
				slog.Debug("refresh failed", "err", err)
				continue
			} else if r != nil && r.Cached {
				continue
			}
			w.mu.Lock()
			w.lastFP = fp
			w.mu.Unlock()
		}
	}
}
func (w *Watcher) pollLoop(ctx context.Context) {
	defer w.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.ticker.C:
			fp := Fingerprint(w.root)
			w.mu.Lock()
			last := w.lastFP
			w.mu.Unlock()
			if fp == last {
				continue
			}
			if r, err := Refresh(w.root, false); err != nil {
				slog.Debug("poll refresh failed", "err", err)
				continue
			} else if r != nil && r.Cached {
				continue
			}
			w.mu.Lock()
			w.lastFP = fp
			w.mu.Unlock()
		}
	}
}
func (w *Watcher) Close() error {
	var err error
	w.once.Do(func() {
		if w.watcher != nil {
			err = w.watcher.Close()
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.timer != nil {
			if !w.timer.Stop() {
				select { case <-w.timer.C: default: }
			}
		}
		if w.ticker != nil {
			w.ticker.Stop()
		}
		clear(w.active)
		clearGlobalIf(w)
	})
	return err
}
func (w *Watcher) IsPolling() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.ticker != nil }
func (w *Watcher) IsWatching() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.watcher != nil && len(w.active) > 0
}

var (
	globalMu      sync.Mutex
	globalWatcher *Watcher
)

func setGlobalWatcher(w *Watcher) { globalMu.Lock(); globalWatcher = w; globalMu.Unlock() }
func clearGlobalIf(w *Watcher) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalWatcher == w {
		globalWatcher = nil
	}
}
func IsPolling() bool {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalWatcher == nil {
		return false
	}
	return globalWatcher.IsPolling()
}
func IsWatching() bool {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalWatcher == nil {
		return false
	}
	return globalWatcher.IsWatching()
}
