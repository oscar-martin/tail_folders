package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io"
	"log"
	"os"
	"os/exec"
	_ "path/filepath"
)

type tailFile struct {
	folder string
}

type rootFolderWatcher struct {
	root          string
	folders       map[string]bool
	tailProcesses map[string]*os.Process
	toStdOutChan  chan<- string
	watcher       *fsnotify.Watcher
}

func makeRootFolderWatcher(root string, toStdOutChan chan<- string) *rootFolderWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	watcher.Add(root)
	return &rootFolderWatcher{
		root:          root,
		folders:       make(map[string]bool),
		tailProcesses: make(map[string]*os.Process),
		toStdOutChan:  toStdOutChan,
		watcher:       watcher,
	}
}

func (r *rootFolderWatcher) watch() {
	go func() {
		for {
			select {
			case event := <-r.watcher.Events:
				if event.Op&fsnotify.Create == fsnotify.Create {
					if stat, err := os.Stat(event.Name); err == nil && stat.IsDir() {
						r.folders[event.Name] = true
						r.watcher.Add(event.Name)
						log.Printf("Added watcher on %s\n", event.Name)
					} else {
						log.Println("Starting tailing...")
						process := tail(event.Name, r.toStdOutChan)
						if process != nil {
							r.tailProcesses[event.Name] = process
						}
					}
				}
				// here we need to keep track of created folders and files in order to be able to know
				// if the removed file was actually a folder or a file
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					_, isDir := r.folders[event.Name]
					if isDir {
						r.watcher.Remove(event.Name)
						delete(r.folders, event.Name)
						log.Printf("Removed watcher on %s\n", event.Name)
					} else {
						process, exists := r.tailProcesses[event.Name]
						if exists {
							err := process.Kill()
							if err != nil {
								log.Printf("Forced removing tail process on %s with error %v\n", event.Name, err)
							} else {
								log.Printf("Forced removing tail process on %s\n", event.Name)
							}
						}
					}
				}
			case err := <-r.watcher.Errors:
				log.Println("Error:", err)
			}
		}
	}()
}

func main() {
	// TODO: Should I support for multiple root paths as parameters?
	rootPath := *flag.String("root", ".", "Description")
	stdoutChan := make(chan string)

	go stdoutWriter(stdoutChan)
	rootFolder := makeRootFolderWatcher(rootPath, stdoutChan)
	defer rootFolder.watcher.Close()

	rootFolder.watch()

	// TODO: Add signal here to support gracefully shutdown
	done := make(chan bool)
	<-done
}

func stdoutWriter(c <-chan string) {
	for {
		select {
		case logMsg := <-c:
			// actual write to stdout
			fmt.Print(logMsg)
		}
	}
}

func prefixingWriter(tag string, toStdOutChan chan<- string) io.Writer {
	pipeReader, pipeWriter := io.Pipe()

	scanner := bufio.NewScanner(pipeReader)
	scanner.Split(bufio.ScanLines)

	go func() {
		for scanner.Scan() {
			toStdOutChan <- fmt.Sprintf("[%s] %s \n", tag, scanner.Bytes())
		}
	}()

	return pipeWriter
}

func tail(filename string, toStdOutChan chan<- string) *os.Process {
	prefixWriter := prefixingWriter(filename, toStdOutChan)

	if stat, err := os.Stat(filename); err == nil && !stat.IsDir() {
		cmd := exec.Command("tail", "-f", filename)
		cmd.Stdout = prefixWriter
		cmd.Stderr = prefixWriter
		err := cmd.Start()
		if err != nil {
			log.Printf("%v\n", err)
		}
		go func(c *exec.Cmd) {
			err := cmd.Wait()
			if err != nil {
				log.Printf("Error: %v\n", err)
			}
		}(cmd)
		return cmd.Process
	}
	log.Printf("Warning! Trying to tail an non-existing file %s. Skipping.\n", filename)
	return nil
}
