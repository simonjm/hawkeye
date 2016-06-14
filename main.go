package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/exp/inotify"
)

var (
	maxJobs = flag.Int("max-jobs", 2, "The max amount of .mkv files that can be processing at once")
	outDir  = flag.String("out-dir", "", "The directory to output mp4 files")
	logFile = flag.String("log-file", "", "The location of the log file")
)

var logger *log.Logger

func main() {
	flag.Parse()

	watchDir := flag.Arg(0)
	if watchDir == "" {
		log.Fatal("The last argument must be a path to the directory to watch")
	}

	if *outDir == "" {
		log.Fatal("--out-dir is required")
	}

	if *logFile == "" {
		log.Fatal("--log-file is required")
	}

	logFile, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)

	pathChan := make(chan string)
	go processFiles(pathChan)

	findInitialFiles(watchDir, pathChan)
	watchDirectory(watchDir, pathChan)
}

func findInitialFiles(watchDir string, pathChan chan string) {
	searchPattern := filepath.Join(watchDir, "*.mkv")
	logger.Printf("Checking %s for initial files with pattern '%s'\n", watchDir, searchPattern)

	matches, err := filepath.Glob(searchPattern)
	if err != nil {
		logger.Println(err)
		return
	}

	for _, m := range matches {
		pathChan <- m
	}
}

func watchDirectory(watchDir string, pathChan chan string) {
	watcher, err := inotify.NewWatcher()
	if err != nil {
		logger.Fatal(err)
	}

	err = watcher.AddWatch(watchDir, inotify.IN_CLOSE_WRITE|inotify.IN_MOVED_TO)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Printf("Started watching for mkv files at %s\n", *outDir)

	for {
		select {
		case ev := <-watcher.Event:
			pathChan <- ev.Name
		case err := <-watcher.Error:
			logger.Println(err)
		}
	}
}

func processFiles(pathChan <-chan string) {
	videosChan := make(chan string, *maxJobs)
	for i := 0; i < *maxJobs; i++ {
		logger.Printf("Worker %d has started\n", i+1)
		go convertFiles(videosChan)
	}

	for p := range pathChan {
		if filepath.Ext(p) == ".mkv" {
			logger.Printf("Queuing %s\n", p)
			videosChan <- p
		}
	}
}

func convertFiles(videosChan <-chan string) {
	for video := range videosChan {
		mp4File := strings.Replace(video, ".mkv", ".mp4", 1)
		output := filepath.Join(*outDir, filepath.Base(mp4File))
		logger.Printf("Converting %s to %s\n", video, output)
		cmd := exec.Command("/usr/bin/ffmpeg", "-i", video, "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", output)
		if err := cmd.Run(); err != nil {
			logger.Println(err)
			continue
		}

		if err := os.Remove(video); err != nil {
			logger.Println(err)
			continue
		}

		logger.Printf("Finished %s\n", output)
	}
}
