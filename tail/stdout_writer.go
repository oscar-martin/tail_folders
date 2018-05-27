package tail

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
)

type OutWriter struct {
	mux *sync.Mutex
	w   io.Writer
}

func MakeOutStringWriter() *OutWriter {
	return &OutWriter{
		mux: &sync.Mutex{},
		w:   bytes.NewBufferString(""),
	}
}

func MakeStdOutWriter() *OutWriter {
	return &OutWriter{
		mux: &sync.Mutex{},
		w:   os.Stdout,
	}
}

func (ow *OutWriter) Start(c <-chan string, tag string) {
	for {
		select {
		case logMsg := <-c:
			// actual write to stdout
			ow.mux.Lock()
			if tag == "" {
				fmt.Fprint(ow.w, logMsg)
			} else {
				fmt.Fprintf(ow.w, "[%s] %s", tag, logMsg)
			}
			ow.mux.Unlock()
		}
	}
}

func (ow *OutWriter) String() string {
	ow.mux.Lock()
	defer ow.mux.Unlock()
	return fmt.Sprintf("%s", ow.w)
}
