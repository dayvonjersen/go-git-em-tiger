package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type event struct {
	filename string
	t        time.Time
}

func dispatch(events chan *event, validator func(string) bool, callback func()) {
	var last time.Time
	for e := range events {
		diff := time.Since(last) - time.Since(e.t)
		last = e.t
		// log.Println("got:", path.Base(e.filename), diff)
		if !validator(e.filename) {
			// log.Println("file is not valid,          skipping...")
			continue
		}
		if diff < time.Millisecond*100 {
			// log.Println("last event was < 100ms ago, skipping...")
			continue
		}
		go callback()
	}
}

func newWatcher(events chan *event) (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case e := <-w.Events:
				events <- &event{
					filename: normalizePathSeparators(e.Name),
					t:        time.Now(),
				}
			case err := <-w.Errors:
				checkErr(err)
			}
		}
	}()

	return w, nil
}

func watchAddWithSubdirs(w *fsnotify.Watcher, watchDir string) (watchPaths []string) {
	watchPaths = []string{}

	watchPath := func(path string) {
		path = normalizePathSeparators(path)
		// log.Println("watching", path)
		watchPaths = append(watchPaths, path)
		checkErr(w.Add(path))
	}

	filepath.Walk(watchDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() && !strings.Contains(path, ".git") {
			watchPath(path)
		}
		return err
	})

	return watchPaths
}

func watchRemove(w *fsnotify.Watcher, watchPaths []string) {
	for _, path := range watchPaths {
		if err := w.Remove(path); err != nil {
			log.Println(err)
		}
	}
}
