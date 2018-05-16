package command

import (
	"log"
	"os/exec"
	"syscall"
	"tail_folders/logger"
)

type logWriter struct {
	logger *log.Logger
}

func newLogWriter(l *log.Logger) *logWriter {
	lw := &logWriter{}
	lw.logger = l
	return lw
}

func (lw logWriter) Write(p []byte) (n int, err error) {
	lw.logger.Printf("%s", p)
	return len(p), nil
}

func ExecuteCommand(command string, args []string) int {
	processLogWriter := newLogWriter(logger.ProcessLog)

	cmd := exec.Command(command, args...)
	cmd.Stdout = processLogWriter
	cmd.Stderr = processLogWriter

	if err := cmd.Start(); err != nil {
		log.Fatalf("Start process '%s' had an error: %v", command, err)
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		} else {
			log.Fatalf("Wait for command '%s' had an error: %v", command, err)
			return -2
		}
	}
	return 0
}
