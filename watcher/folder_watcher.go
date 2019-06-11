package watcher

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/oscar-martin/tail_folders/logger"
	"github.com/oscar-martin/tail_folders/tail"

	"github.com/fsnotify/fsnotify"
)

type rootFolderWatcher struct {
	// root folder
	root string
	// exitChans contains the channel that exits processing goroutines for watcher events
	exitChans map[string]chan<- struct{}
	// tailProcesses contains the tail process that are running per folder
	tailProcesses map[string]map[string]*os.Process
	// watchers contains the watcher instance per subfolder
	watchers map[string]*fsnotify.Watcher
	// toStsdOutChan is the channel to use for outputing the tail information from files
	toStdOutChan chan<- tail.Entry
	recursive    bool
	filterFunc   func(string) bool
	timeout      int
	oldFiles     int
}

// MakeRootFolderWatcher lets you create a rootFolderWatcher instance
func MakeRootFolderWatcher(root string, toStdOutChan chan<- tail.Entry, recursive bool, filterFunc func(string) bool, timeout, oldFiles int) *rootFolderWatcher {
	return &rootFolderWatcher{
		root:          root,
		exitChans:     make(map[string]chan<- struct{}),
		tailProcesses: make(map[string]map[string]*os.Process),
		watchers:      make(map[string]*fsnotify.Watcher),
		toStdOutChan:  toStdOutChan,
		recursive:     recursive,
		filterFunc:    filterFunc,
		timeout:       timeout,
		oldFiles:      oldFiles,
	}
}

func (r *rootFolderWatcher) scanAndAddSubfolder(folderPath string, dataChan chan<- tail.Entry) {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		log.Fatalf("Error trying to scan folder path '%s': %v", folderPath, err)
	} else {
		for _, fileInfo := range files {
			filename := path.Join(folderPath, fileInfo.Name())
			r.processExistingFileInfo(folderPath, fileInfo, filename, dataChan)
		}
	}
}

func (r *rootFolderWatcher) processExistingFileInfo(folder string, fileInfo os.FileInfo, filename string, dataChan chan<- tail.Entry) {
	if fileInfo.IsDir() && !isHidden(filename) && r.recursive {
		r.watch(filename)
	} else {
		if r.filterFunc(fileInfo.Name()) {
			modTime := fileInfo.ModTime()
			diff := time.Now().Sub(modTime)
			if r.oldFiles > 0 && diff.Seconds() > float64(r.oldFiles) {
				logger.Info.Printf("Discarding tailing file '%s' because it is too old\n", filename)
				return
			}

			process, err := tail.DoTail(filename, dataChan)
			if err != nil {
				logger.Error.Printf("Error trying to tail file '%s': %v", filename, err)
				return
			}
			logger.Info.Printf("Started tailing '%s'\n", filename)
			if process != nil {
				if _, ok := r.tailProcesses[folder]; !ok {
					r.tailProcesses[folder] = make(map[string]*os.Process)
				}
				r.tailProcesses[folder][filename] = process
			}
		}
	}
}

func (r *rootFolderWatcher) processDeletedFile(folder string, name string) {
	if processes, ok := r.tailProcesses[folder]; ok {
		if process, ok := processes[name]; ok {
			err := process.Kill()
			if err != nil {
				logger.Error.Printf("Forced removing tail process on '%s' (file is removed) with error %v\n", folder, err)
			} else {
				logger.Info.Printf("Forced removing tail process on '%s' because file has been removed\n", folder)
			}
		} else {
			logger.Warning.Printf("tail process for '%s' is not found\n", name)
		}
	}
}

func (r *rootFolderWatcher) Close() {
	for folder, watcher := range r.watchers {
		logger.Info.Printf("Watcher on folder '%s' closed\n", folder)
		watcher.Close()
	}

	for folder, exitChan := range r.exitChans {
		logger.Info.Printf("Processor on folder '%s' terminated\n", folder)
		close(exitChan)
	}
}

func (r *rootFolderWatcher) unwatch(folder string) error {
	// only unwatch subfolders... root folder must remain watching
	if r.root != folder {
		if watcher, ok := r.watchers[folder]; ok {
			watcher.Close()
			delete(r.watchers, folder)
			logger.Info.Printf("Removed watcher of folder '%s'\n", folder)
		}
		if exitChan, ok := r.exitChans[folder]; ok {
			close(exitChan)
			delete(r.exitChans, folder)
			logger.Info.Printf("Closing processor for events in folder '%s'\n", folder)
		}
	}
	if processes, ok := r.tailProcesses[folder]; ok {
		for _, process := range processes {
			err := process.Kill()
			if err != nil {
				logger.Error.Printf("Forced removing tail process on '%s' (file is removed) with error %v\n", folder, err)
			} else {
				logger.Info.Printf("Forced removing tail process on '%s' because file has been removed\n", folder)
			}
		}
	}
	return nil
}

func (r *rootFolderWatcher) watch(folder string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// scan current folders (whether recursive flag is enabled) and files
	dataChan := make(chan tail.Entry)
	r.scanAndAddSubfolder(folder, dataChan)

	exitChan := make(chan struct{})
	activityChan := make(chan struct{})
	r.exitChans[folder] = exitChan
	r.watchers[folder] = watcher
	watcher.Add(folder)
	logger.Info.Printf("Added watcher for '%s'\n", folder)

	trackActivity := r.timeout > 0

	go func() {
		for {
			select {
			case entry := <-dataChan:
				r.toStdOutChan <- entry
				if trackActivity {
					// notify new activity
					activityChan <- struct{}{}
				}
			case <-exitChan:
				return
			}
		}
	}()

	// now, run the watcher
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// fmt.Printf("%v \n", event)
				if event.Op&fsnotify.Create == fsnotify.Create {
					fileInfo, err := os.Stat(event.Name)
					if err != nil {
						logger.Error.Printf("Unable to stat file '%s': %v", event.Name, err)
					} else {
						r.processExistingFileInfo(folder, fileInfo, event.Name, dataChan)
					}
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					// fmt.Printf("%v \n", event)
					if folder == event.Name {
						r.unwatch(folder)
					} else {
						r.processDeletedFile(folder, event.Name)
					}
				}
			case err := <-watcher.Errors:
				if err != nil {
					logger.Error.Printf("Error: %v\n", err)
				}
			case <-exitChan:
				return
			}
		}
	}()
	logger.Info.Printf("Start watching on folder '%s'\n", folder)

	if trackActivity {
		timer := time.NewTimer(time.Duration(r.timeout) * time.Second)
		go func() {
			for {
				select {
				case <-timer.C:
					logger.Info.Printf("Inactivity timeout set off for folder '%s'\n", folder)
					r.unwatch(folder)
				case <-activityChan:
					if !timer.Stop() {
						<-timer.C
					}
					timer.Reset(time.Duration(r.timeout) * time.Second)
				case <-exitChan:
					return
				}
			}
		}()
		logger.Info.Printf("Start activity monitor on folder '%s'\n", folder)
	}

	return nil
}

func (r *rootFolderWatcher) Watch() error {
	return r.watch(r.root)
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
