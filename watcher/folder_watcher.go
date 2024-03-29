package watcher

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/oscar-martin/tail_folders/logger"
	"github.com/oscar-martin/tail_folders/tail"

	"github.com/fsnotify/fsnotify"
)

type rootFolderWatcher struct {
	// mutex to protect shared resources from different goroutines
	mutex sync.Mutex
	// root folder
	root string
	// exitChans contains the channel that exits processing goroutines for watcher events
	exitChans map[string]chan<- struct{}
	// tailProcesses contains the tail process that are running per folder
	tailProcesses map[string]map[string]*os.Process
	// watchers contains the watcher instance per subfolder
	watchers map[string]*fsnotify.Watcher
	// toStsdOutChan is the channel to use for outputing the tail information from files
	toStdOutChan      chan<- tail.Entry
	recursive         bool
	filterFunc        func(string) bool
	contentFilterFunc func(string) bool
	timeout           int
	oldFiles          int
}

// MakeRootFolderWatcher lets you create a rootFolderWatcher instance
func MakeRootFolderWatcher(root string, toStdOutChan chan<- tail.Entry, recursive bool, filterFunc func(string) bool, contentFilterFunc func(string) bool, timeout, oldFiles int) *rootFolderWatcher {
	return &rootFolderWatcher{
		root:              root,
		exitChans:         make(map[string]chan<- struct{}),
		tailProcesses:     make(map[string]map[string]*os.Process),
		watchers:          make(map[string]*fsnotify.Watcher),
		toStdOutChan:      toStdOutChan,
		recursive:         recursive,
		filterFunc:        filterFunc,
		contentFilterFunc: contentFilterFunc,
		timeout:           timeout,
		oldFiles:          oldFiles,
	}
}

func (r *rootFolderWatcher) scanAndAddSubfolder(folderPath string, dataChan chan<- tail.Entry) error {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return err
	}
	for _, fileInfo := range files {
		filename := path.Join(folderPath, fileInfo.Name())
		r.processExistingFileInfo(folderPath, fileInfo, filename, dataChan)
	}

	return nil
}

func (r *rootFolderWatcher) processExistingFileInfo(folder string, fileInfo os.FileInfo, filename string, dataChan chan<- tail.Entry) {
	if fileInfo.IsDir() && !isHidden(filename) && r.recursive {
		err := r.watch(filename)
		if err != nil {
			logger.Error.Printf("Error trying to watch folder path '%s': %v. Skipping...", folder, err)
			return
		}
	} else {
		if r.filterFunc(fileInfo.Name()) {
			modTime := fileInfo.ModTime()
			diff := time.Now().Sub(modTime)
			if r.oldFiles > 0 && diff.Seconds() > float64(r.oldFiles) {
				logger.Info.Printf("Discarding tailing file '%s' because it is too old\n", filename)
				return
			}

			process, err := tail.DoTail(filename, dataChan, r.contentFilterFunc)
			if err != nil {
				logger.Error.Printf("Error trying to tail file '%s': %v", filename, err)
				return
			}
			logger.Info.Printf("Started tailing '%s'\n", filename)
			if process != nil {
				r.mutex.Lock()
				if _, ok := r.tailProcesses[folder]; !ok {
					r.tailProcesses[folder] = make(map[string]*os.Process)
				}
				r.tailProcesses[folder][filename] = process
				r.mutex.Unlock()
			}
		}
	}
}

func (r *rootFolderWatcher) processDeletedFile(folder string, name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
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
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for folder, watcher := range r.watchers {
		logger.Info.Printf("Watcher on folder '%s' closed\n", folder)
		watcher.Close()
	}

	for folder, exitChan := range r.exitChans {
		logger.Info.Printf("Processor on folder '%s' terminated\n", folder)
		close(exitChan)
	}
}

func (r *rootFolderWatcher) unwatch(folder string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
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
}

func (r *rootFolderWatcher) watch(folder string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// scan current folders (whether recursive flag is enabled) and files
	dataChan := make(chan tail.Entry)
	activityChan := make(chan struct{})

	err = r.scanAndAddSubfolder(folder, dataChan)
	if err != nil {
		close(dataChan)
		close(activityChan)
		return err
	}

	exitChan := make(chan struct{})
	r.mutex.Lock()
	r.exitChans[folder] = exitChan
	r.watchers[folder] = watcher
	r.mutex.Unlock()
	_ = watcher.Add(folder)
	logger.Info.Printf("Added watcher for '%s'\n", folder)

	trackActivity := r.timeout > 0

	// this receives data coming from any file within this folder
	go func() {
		for {
			select {
			case entry, ok := <-dataChan:
				if ok {
					r.toStdOutChan <- entry
					if trackActivity {
						// notify new activity
						activityChan <- struct{}{}
					}
				}
			case <-exitChan:
				return
			}
		}
	}()

	// run watcher processor
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
		// this takes care of tracking activity
		go func() {
			for {
				select {
				case <-timer.C:
					logger.Info.Printf("Inactivity timeout set off for folder '%s'\n", folder)
					r.unwatch(folder)
				case _, ok := <-activityChan:
					if ok {
						if !timer.Stop() {
							<-timer.C
						}
						timer.Reset(time.Duration(r.timeout) * time.Second)
					}
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
	basename := filepath.Base(filename)
	if runtime.GOOS != "windows" {
		// unix/linux file or directory that starts with . is hidden
		if strings.HasPrefix(basename, ".") {
			return true
		}
	}
	return false
}
