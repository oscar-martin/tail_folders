package tail

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/oscar-martin/tail_folders/logger"
)

func prefixingWriter(tag string, toStdOutChan chan<- string) io.Writer {
	pipeReader, pipeWriter := io.Pipe()

	scanner := bufio.NewScanner(pipeReader)
	scanner.Split(bufio.ScanLines)

	go func() {
		for scanner.Scan() {
			toStdOutChan <- fmt.Sprintf("[%s] %s\n", tag, scanner.Bytes())
		}
	}()

	return pipeWriter
}

func DoTail(filename string, toStdOutChan chan<- string) *os.Process {
	prefixWriter := prefixingWriter(filename, toStdOutChan)

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
		return cmd.Process
	}
	logger.Warning.Printf("Trying to tail an non-existing file %s. Skipping.\n", filename)
	return nil
}
