package tail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/oscar-martin/tail_folders/logger"
)

type entryToStringF func(e Entry, tag string) (string, error)

func EntryToRawString(e Entry, tag string) (string, error) {
	if tag == "" {
		return fmt.Sprintf("[%s] %s", e.File, e.Message), nil
	}
	return fmt.Sprintf("[%s] [%s] %s", tag, e.File, e.Message), nil
}

func EntryToJsonString(e Entry, tag string) (string, error) {
	e.Tag = tag
	bytes, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type OutWriter struct {
	mux      *sync.Mutex
	w        io.Writer
	toString entryToStringF
}

func MakeOutStringWriter() *OutWriter {
	return &OutWriter{
		mux:      &sync.Mutex{},
		w:        bytes.NewBufferString(""),
		toString: EntryToRawString,
	}
}

func MakeStdOutWriter(toStringF entryToStringF) *OutWriter {
	return &OutWriter{
		mux:      &sync.Mutex{},
		w:        os.Stdout,
		toString: toStringF,
	}
}

func (ow *OutWriter) Start(c <-chan Entry, tag string) {
	for {
		select {
		case entry := <-c:
			// actual write to out
			str, err := ow.toString(entry, tag)
			if err != nil {
				fmt.Printf("%v\n", err)
				logger.Error.Printf("%v\n", err)
			} else {
				ow.mux.Lock()
				fmt.Fprintln(ow.w, str)
				ow.mux.Unlock()
			}
		}
	}
}

func (ow *OutWriter) String() string {
	ow.mux.Lock()
	defer ow.mux.Unlock()
	return fmt.Sprintf("%s", ow.w)
}
