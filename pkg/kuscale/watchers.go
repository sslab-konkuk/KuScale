package kuscale

import (
	"os"
	"os/signal"

	"github.com/fsnotify/fsnotify"
	// "context"
	// "fmt"
	// "net"
	// "time"
	// "path"
	// "google.golang.org/grpc"
	// podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
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

func newOSWatcher(sigs ...os.Signal) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sigs...)

	return sigChan
}
