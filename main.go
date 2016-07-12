package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

		commandArgs := []string{"-y", "-i", video, "-c:v", "copy", "-c:a"}
		shouldConvert, err := shouldConvertAudio(video)
		if err != nil {
			logger.Println(err)
			continue
		}

		if shouldConvert {
			commandArgs = append(commandArgs, "aac", "-b:a", "192k", output)
		} else {
			commandArgs = append(commandArgs, "copy", output)
		}

		logger.Printf("Running ffmpeg with arguments %v", commandArgs)
		if _, err := runCommandWithErrorMsg("/usr/bin/ffmpeg", commandArgs...); err != nil {
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

// Runs the command and returns the stdout or an error with stderr as the message
func runCommandWithErrorMsg(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	outBuf := bytes.Buffer{}
	if _, err := io.Copy(&outBuf, stdout); err != nil {
		return "", err
	}

	errBuf := bytes.Buffer{}
	if _, err := io.Copy(&errBuf, stderr); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("%s. Stderr - %s", err, errBuf.String())
	}

	return outBuf.String(), nil
}

// Checks if video needs the audio converted to AAC
func shouldConvertAudio(filename string) (bool, error) {
	probeCommand := fmt.Sprintf("ffprobe -v error -show_format -show_streams \"%s\" | grep codec_name", filename)
	output, err := runCommandWithErrorMsg("/bin/sh", "-c", probeCommand)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(output), "\n")
	for _, l := range lines {
		matched, err := regexp.Match("codec_name=aac", []byte(l))
		if err != nil {
			return false, err
		}

		if matched {
			return false, nil
		}
	}

	return true, nil
}
