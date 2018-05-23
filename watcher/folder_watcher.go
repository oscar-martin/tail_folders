package watcher

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"tail_folders/logger"
	"tail_folders/tail"

	"github.com/fsnotify/fsnotify"
)

type rootFolderWatcher struct {
	root          string
	folders       map[string]bool
	tailProcesses map[string]*os.Process
	toStdOutChan  chan<- string
	watcher       *fsnotify.Watcher
	recursive     bool
	filterFunc    func(string) bool
}

func MakeRootFolderWatcher(root string, toStdOutChan chan<- string, recursive bool, filterFunc func(string) bool) *rootFolderWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error.Fatal(err)
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
	if fileInfo.IsDir() && !isHidden(filename) && r.recursive {
		r.folders[filename] = true
		r.watcher.Add(filename)
		logger.Info.Printf("Added folder '%s' on watcher\n", filename)
		if r.recursive {
			// Try to add any nested folder that could've created...
			r.scanAndAddSubfolder(filename)
		}
	} else {
		if r.filterFunc(fileInfo.Name()) {
			process := tail.DoTail(filename, r.toStdOutChan)
			logger.Info.Printf("Started tailing '%s'\n", fileInfo.Name())
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
		logger.Info.Printf("Removed tracking of folder '%s' on watcher\n", name)
	} else {
		process, exists := r.tailProcesses[name]
		if exists {
			err := process.Kill()
			if err != nil {
				logger.Error.Printf("Forced removing tail process on '%s' (file is removed) with error %v\n", name, err)
			} else {
				logger.Info.Printf("Forced removing tail process on '%s' because file has been removed\n", name)
			}
		}
	}
}

func (r *rootFolderWatcher) Close() {
	r.watcher.Close()
	logger.Info.Printf("Watcher on folder '%s' closed\n", r.root)
}

func (r *rootFolderWatcher) Watch() {
	// scan current folders (whether recursive flag is enabled) and files
	r.scanAndAddSubfolder(r.root)

	// now, run the watcher
	go func() {
		for {
			select {
			case event := <-r.watcher.Events:
				// Info.Printf("%v \n", event)
				if event.Op&fsnotify.Create == fsnotify.Create {
					fileInfo, err := os.Stat(event.Name)
					if err != nil {
						logger.Error.Printf("Unable to stat file '%s': %v", event.Name, err)
					} else {
						filename := fileInfo.Name()
						r.processExistingFileInfo(fileInfo, filename)
					}
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					r.processDeletedFileOrFolder(event.Name)
				}
			case err := <-r.watcher.Errors:
				if err != nil {
					logger.Error.Printf("Error: %v\n", err)
				}
			}
		}
	}()
	logger.Info.Printf("Start watching on folder '%s'\n", r.root)
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
