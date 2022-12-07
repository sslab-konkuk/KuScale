package kuwatcher

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

func newFSWatcher(files ...string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		err = watcher.Add(f)
		if err != nil {
			watcher.Close()
			return nil, err
		}
	}

	return watcher, nil
}

func SignalWatcher() chan string {
	stopCh := make(chan string)
	shutdownSignals := []os.Signal{os.Interrupt, syscall.SIGTERM}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)
	go func() {
		<-sigCh
		close(stopCh)
		<-sigCh
		os.Exit(1)
	}()
	return stopCh
}
