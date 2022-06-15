package tail

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/oscar-martin/tail_folders/logger"
)

type acceptFunc func(string) bool

// Entry models a line read from a source file
type Entry struct {
	// Tag is user-provided setting for different tail_folders processes running
	// in a single host
	Tag string `json:"tag,omitempty"`
	// Hostname is the hostname where tail_folders is running
	Hostname string `json:"host,omitempty"`
	// Folders is a list of folder names where the source file is
	Folders []string `json:"dirs,omitempty"`
	// Filename is the base filename of the source file
	Filename string `json:"file,omitempty"`
	// File is the whole filepath. This field is for internal use only
	File string `json:"-"`
	// Message is the actual payload read from the source file
	Message string `json:"msg,omitempty"`
	// Timestamp is the time where the log is read
	Timestamp time.Time `json:"time,omitempty"`
}

func lineProcessorWriter(fpath string, toEntryChan chan<- Entry, acceptF acceptFunc) (io.Writer, error) {
	pipeReader, pipeWriter := io.Pipe()

	scanner := bufio.NewScanner(pipeReader)
	scanner.Split(bufio.ScanLines)

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	dir, file := filepath.Split(fpath)
	allFolders := strings.Split(dir, string(os.PathSeparator))
	folders := []string{}

	for _, folder := range allFolders {
		if folder != "" {
			folders = append(folders, folder)
		}
	}

	go func(host string, folders []string, file string) {
		for scanner.Scan() {
			message := string(scanner.Bytes())
			if acceptF(message) {
				now := time.Now()
				entry := Entry{
					Folders:   folders,
					Message:   message,
					Timestamp: now,
					File:      fpath,
					Filename:  file,
					Hostname:  hostname,
				}
				toEntryChan <- entry
			}
		}
	}(hostname, folders, file)

	return pipeWriter, nil
}

func DoTail(filename string, toEntryChan chan<- Entry, acceptF acceptFunc) (*os.Process, error) {
	prefixWriter, err := lineProcessorWriter(filename, toEntryChan, acceptF)
	if err != nil {
		return nil, err
	}

	if stat, err := os.Stat(filename); err == nil && !stat.IsDir() {
		cmd := exec.Command("tail", "-f", "-n", "0", filename)
		cmd.Stdout = prefixWriter
		cmd.Stderr = prefixWriter
		err := cmd.Start()
		if err != nil {
			logger.Error.Printf("%v\n", err)
		}
		go func(c *exec.Cmd) {
			err := cmd.Wait()
			if err != nil {
				logger.Warning.Printf("%s -> %v\n", filename, err)
			}
		}(cmd)
		return cmd.Process, nil
	}
	logger.Warning.Printf("Trying to tail an non-existing file %s. Skipping.\n", filename)
	return nil, nil
}
