package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	Info          *log.Logger
	Warning       *log.Logger
	Error         *log.Logger
	logFolderName = "./.logdir"
)

func initLogs(
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	Info = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(warningHandle, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

type rootFolderWatcher struct {
	root          string
	folders       map[string]bool
	tailProcesses map[string]*os.Process
	toStdOutChan  chan<- string
	watcher       *fsnotify.Watcher
	recursive     bool
	filterFunc    func(string) bool
}

func makeRootFolderWatcher(root string, toStdOutChan chan<- string, recursive bool, filterFunc func(string) bool) *rootFolderWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Error.Fatal(err)
	}

	watcher.Add(root)
	return &rootFolderWatcher{
		root:          root,
		folders:       make(map[string]bool),
		tailProcesses: make(map[string]*os.Process),
		toStdOutChan:  toStdOutChan,
		watcher:       watcher,
		recursive:     recursive,
		filterFunc:    filterFunc,
	}
}

func (r *rootFolderWatcher) scanAndAddSubfolder(folderPath string) {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		log.Fatalf("Error trying to scan folder path '%s': %v", folderPath, err)
	} else {
		for _, fileInfo := range files {
			filename := path.Join(folderPath, fileInfo.Name())
			r.processExistingFileInfo(fileInfo, filename)
		}
	}
}

func (r *rootFolderWatcher) processExistingFileInfo(fileInfo os.FileInfo, filename string) {
	if fileInfo.IsDir() && !isHidden(filename) {
		r.folders[filename] = true
		r.watcher.Add(filename)
		Info.Printf("Added folder '%s' on watcher\n", filename)
		if r.recursive {
			// Try to add any nested folder that could've created...
			r.scanAndAddSubfolder(filename)
		}
	} else {
		if r.filterFunc(fileInfo.Name()) {
			process := tail(filename, r.toStdOutChan)
			Info.Printf("Started tailing '%s'\n", fileInfo.Name())
			if process != nil {
				r.tailProcesses[filename] = process
			}
		}
	}
}

func (r *rootFolderWatcher) processDeletedFileOrFolder(name string) {
	_, isDir := r.folders[name]
	if isDir {
		r.watcher.Remove(name)
		delete(r.folders, name)
		Info.Printf("Removed tracking of folder '%s' on watcher\n", name)
	} else {
		process, exists := r.tailProcesses[name]
		if exists {
			err := process.Kill()
			if err != nil {
				Error.Printf("Forced removing tail process on '%s' (file is removed) with error %v\n", name, err)
			} else {
				Info.Printf("Forced removing tail process on '%s' because file has been removed\n", name)
			}
		}
	}
}

func (r *rootFolderWatcher) close() {
	r.watcher.Close()
	Info.Printf("Watcher on folder '%s' closed\n", r.root)
}

func (r *rootFolderWatcher) watch() {
	// scan current folders (whether recursive flag is enabled) and files
	r.scanAndAddSubfolder(r.root)

	// now, run the watcher
	go func() {
		for {
			select {
			case event := <-r.watcher.Events:
				Info.Printf("%v \n", event)
				if event.Op&fsnotify.Create == fsnotify.Create {
					fileInfo, err := os.Stat(event.Name)
					if err != nil {
						Error.Printf("Unable to stat file '%s': %v", event.Name, err)
					} else {
						filename := fileInfo.Name()
						r.processExistingFileInfo(fileInfo, filename)
					}
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					r.processDeletedFileOrFolder(event.Name)
				}
			case err := <-r.watcher.Errors:
				Error.Println("Error:", err)
			}
		}
	}()
	Info.Printf("Start watching on folder '%s'\n", r.root)
}

func main() {
	// processing command arguments
	folderPathsPtr := flag.String("folders", ".", "Paths of the folders to watch for log files, separated by comma (,). IT SHOULD NOT BE NESTED. Defaults to current directory")
	recursivePtr := flag.Bool("recursive", true, "Whether or not recursive folders should be watched")
	expressionTypePtr := flag.String("filter_by", "glob", "Expression type: Either 'glob' or 'regex'. Defaults to 'glob'")
	filterPtr := flag.String("filter", "*.log", "Filter expression to apply on filenames")
	tagPtr := flag.String("tag", "", "Optional tag to use for each line")

	flag.Usage = func() {
		fmt.Printf("%s - %s", os.Args[0], "Application that scans a list of folders (recursively by default) and tails any file that matches the filename filter\n\n")
		fmt.Printf("Usage of %s <options>\n", os.Args[0])
		fmt.Println("")
		flag.PrintDefaults()
	}
	flag.Parse()

	folderPathsStr := strings.TrimSpace(*folderPathsPtr)
	expressionTypeStr := strings.TrimSpace(*expressionTypePtr)
	filterStr := strings.TrimSpace(*filterPtr)
	tagStr := strings.TrimSpace(*tagPtr)

	// create filename filter
	filterFunc, err := createFilterFunc(expressionTypeStr, filterStr)
	if err != nil {
		log.Fatalf("Unrecognized filter_by value: %s", expressionTypeStr)
	}

	// initialize loggers
	logFile := createLogFile()
	initLogs(logFile, logFile, logFile)

	Info.Println("Arguments in place:")
	Info.Printf("- folders: %s", folderPathsStr)
	Info.Printf("- recursive: %v", *recursivePtr)
	Info.Printf("- filter_by: %s", expressionTypeStr)
	Info.Printf("- filter: %s", filterStr)
	Info.Printf("- tag: %s", strings.TrimSpace(tagStr))

	// init program
	stdoutChan := make(chan string)
	go stdoutWriter(stdoutChan, tagStr)

	for _, folderPath := range strings.Split(folderPathsStr, ",") {
		rootFolderWatcher := makeRootFolderWatcher(folderPath, stdoutChan, *recursivePtr, filterFunc)
		defer rootFolderWatcher.close()
		rootFolderWatcher.watch()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// block here until signal is received
	<-stop
}

func createFilterFunc(expressionTypeStr string, filterStr string) (func(string) bool, error) {
	var filterFunc func(string) bool
	switch expressionTypeStr {
	case "glob":
		filterFunc = filterByGlob(filterStr)
	case "regex":
		filterFunc = filterByRegex(filterStr)
	default:
		return nil, fmt.Errorf("Unrecognized filter_by value: %s", expressionTypeStr)
	}
	return filterFunc, nil
}

func createLogFile() *os.File {
	// create the log folder and file. It is inside a hidden folder for avoiding being tracked itself
	logFolder := path.Join(".", logFolderName)
	os.MkdirAll(logFolder, os.ModePerm)

	logFileName := path.Join(logFolder, "taillog.log")
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to open log file ", logFileName, ":", err)
	}
	return logFile
}

func filterByGlob(globPattern string) func(string) bool {
	_, err := filepath.Match(globPattern, "text.txt")
	if err != nil {
		log.Fatalf("Globbing Expression '%s' is not right: %v", globPattern, err)
	}

	return func(filename string) bool {
		matched, _ := filepath.Match(globPattern, filename)
		return matched
	}
}

func filterByRegex(regexStr string) func(string) bool {
	regex := regexp.MustCompile(regexStr)
	return func(filename string) bool {
		return regex.MatchString(filename)
	}
}

func stdoutWriter(c <-chan string, tag string) {
	for {
		select {
		case logMsg := <-c:
			// actual write to stdout
			if tag == "" {
				fmt.Print(logMsg)
			} else {
				fmt.Printf("[%s] %s", tag, logMsg)
			}
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
		cmd := exec.Command("tail", "-f", "-n", "0", filename)
		cmd.Stdout = prefixWriter
		cmd.Stderr = prefixWriter
		err := cmd.Start()
		if err != nil {
			Error.Printf("%v\n", err)
		}
		go func(c *exec.Cmd) {
			err := cmd.Wait()
			if err != nil {
				Warning.Printf("%s -> %v\n", filename, err)
			}
		}(cmd)
		return cmd.Process
	}
	Warning.Printf("Trying to tail an non-existing file %s. Skipping.\n", filename)
	return nil
}

func isHidden(filename string) bool {
	if runtime.GOOS != "windows" {
		// unix/linux file or directory that starts with . is hidden
		if filename[0:1] == "." {
			return true
		}
	}
	return false
}
