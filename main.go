package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
	"simongeeks.com/joe/hawkeye/inotify"
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

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatal(err)
	}

	var logWriter io.Writer
	if *logFile == "" {
		logWriter = os.Stdout
	} else {
		logFile, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			log.Fatal(err)
		}
		defer logFile.Close()
		logWriter = logFile
	}

	logger = log.New(logWriter, "", log.Ldate|log.Ltime|log.Lshortfile)

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

	err = watcher.Add(watchDir, unix.IN_CLOSE_WRITE|unix.IN_MOVED_TO)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Printf("Started watching for mkv files at %s\n", watchDir)

	for {
		select {
		case ev := <-watcher.Events:
			pathChan <- ev.Name
		case err := <-watcher.Errors:
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
		codecs, err := getCodecs(video)
		if err != nil {
			logger.Println(err)
			continue
		}

		// h264 videos are supported and its too slow to convert videos to that on a raspberry pi
		if !hasCodec("h264", codecs) {
			logger.Printf("Codec not supported %s\n", video)
			continue
		}

		// check if we need to convert the audio
		if !hasCodec("aac", codecs) {
			commandArgs = append(commandArgs, "aac", "-b:a", "192k", output)
		} else {
			commandArgs = append(commandArgs, "copy", output)
		}

		logger.Printf("Running ffmpeg with arguments %v\n", commandArgs)
		if err := exec.Command("/usr/bin/ffmpeg", commandArgs...).Run(); err != nil {
			logger.Println(err)
			continue
		}

		// delete the old video
		if err := os.Remove(video); err != nil {
			logger.Println(err)
			continue
		}

		logger.Printf("Finished %s\n", output)
	}
}

// Checks if a specific codec is in the list
func hasCodec(codec string, codecs []string) bool {
	for _, c := range codecs {
		if c == codec {
			return false
		}
	}

	return true
}

// Gets a lit of codecs that are used by the video file
func getCodecs(filename string) ([]string, error) {
	probeCommand := fmt.Sprintf("ffprobe -v error -show_format -show_streams \"%s\" | grep codec_name", filename)
	output, err := exec.Command("/bin/sh", "-c", probeCommand).Output()
	if err != nil {
		return nil, err
	}

	codecs := make([]string, 0, 2)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	regex := regexp.MustCompile("codec_name=(.+)$")
	for _, l := range lines {
		matches := regex.FindAllStringSubmatch(l, -1)
		if matches == nil {
			return nil, errors.New("Could not parse codecs")
		}
		codecs = append(codecs, strings.TrimSpace(matches[0][1]))
	}

	return codecs, nil
}
