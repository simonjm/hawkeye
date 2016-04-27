package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"golang.org/x/exp/inotify"
)

var (
	maxJobs = flag.Int("max-jobs", 2, "The max amount of .mkv files that can be processing at once")
	outDir  = flag.String("out-dir", "", "The directory to output mp4 files")
)

func main() {
	flag.Parse()

	if *outDir == "" {
		log.Fatal("--out-dir is required")
	}

	pathChan := make(chan string)
	go processFiles(pathChan)

	watcher, err := inotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.AddWatch(*outDir, inotify.IN_CREATE|inotify.IN_MOVED_TO)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case ev := <-watcher.Event:
			pathChan <- ev.Name
		case err := <-watcher.Error:
			log.Fatal(err)
		}
	}
}

func processFiles(pathChan <-chan string) {
	videosChan := make(chan string, *maxJobs)
	for i := 0; i < *maxJobs; i++ {
		go convertFiles(videosChan)
	}

	for p := range pathChan {
		if filepath.Ext(p) == ".mkv" {
			videosChan <- p
		}
	}
}

func convertFiles(videosChan <-chan string) {
	for video := range videosChan {
		fmt.Printf("Converting mkv %s\n", video)
		time.Sleep(30 * time.Second)
	}
}
