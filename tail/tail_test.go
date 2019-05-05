package tail

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

const (
	One = "One\n"
	Two = "Two\n"
	Tag = "aTag"
)

func Example() {
	chanOut := make(chan Entry)

	writer, _ := lineProcessorWriter(Tag, chanOut)

	go func() {
		writer.Write([]byte(One))
		writer.Write([]byte(Two))
	}()

	time.Sleep(10 * time.Millisecond)
	exitChan := make(chan struct{})
	go func() {
		for {
			select {
			case e := <-chanOut:
				fmt.Println(e.Message)
			case <-exitChan:
				return
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	exitChan <- struct{}{}
	// Output:
	// One
	// Two
}

func TestDoTail(t *testing.T) {
	content := []byte("temporary file's content\n")
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(tmpfile.Name()) // clean up

	chanOut := make(chan Entry)
	tailProcess, _ := DoTail(tmpfile.Name(), chanOut)

	time.Sleep(100 * time.Millisecond)
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}

	readContent := <-chanOut

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}
	expectedContent := Entry{Message: string(content), File: tmpfile.Name()} // fmt.Sprintf("[%s] %s", tmpfile.Name(), content)
	if !reflect.DeepEqual(readContent.File, expectedContent.File) &&
		!reflect.DeepEqual(readContent.Message, expectedContent.Message) {
		t.Errorf("Found content %v is not expected; wanted %v", readContent, expectedContent)
	}

	if err := tailProcess.Kill(); err != nil {
		t.Fatal(err)
	}
}
