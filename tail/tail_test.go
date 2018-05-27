package tail

import "fmt"
import "time"
import "testing"
import "io/ioutil"
import "os"
import "strings"

const (
	One = "One\n"
	Two = "Two\n"
	Tag = "aTag"
)

func Example() {
	chanOut := make(chan string)

	writer := prefixingWriter(Tag, chanOut)

	go func() {
		writer.Write([]byte(One))
		writer.Write([]byte(Two))
	}()

	time.Sleep(10 * time.Millisecond)
	exitChan := make(chan struct{})
	go func() {
		for {
			select {
			case str := <-chanOut:
				fmt.Print(str)
			case <-exitChan:
				return
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	exitChan <- struct{}{}
	// Output:
	// [aTag] One
	// [aTag] Two
}

func TestDoTail(t *testing.T) {
	content := []byte("temporary file's content\n")
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(tmpfile.Name()) // clean up

	chanOut := make(chan string)
	tailProcess := DoTail(tmpfile.Name(), chanOut)

	time.Sleep(100 * time.Millisecond)
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}

	readContent := <-chanOut

	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}
	expectedContent := fmt.Sprintf("[%s] %s", tmpfile.Name(), content)
	if strings.Compare(readContent, string(expectedContent)) != 0 {
		t.Error("Expected content is not received")
	}

	if err := tailProcess.Kill(); err != nil {
		t.Fatal(err)
	}
}
