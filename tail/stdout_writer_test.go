package tail

import (
	"testing"
	"time"
)

func TestStdoutWriterWithoutTag(t *testing.T) {
	c := make(chan Entry)
	outWriter := MakeOutStringWriter()
	go outWriter.Start(c, "")
	c <- Entry{File: "file.txt", Message: "One"}
	c <- Entry{File: "file.txt", Message: "Two"}
	time.Sleep(100 * time.Millisecond)

	wanted := "[file.txt] One\n[file.txt] Two\n"
	if outWriter.String() != wanted {
		t.Errorf("Found: %s; wanted: %s", outWriter.String(), wanted)
	}
}

func TestStdoutWriterWithTag(t *testing.T) {
	c := make(chan Entry)
	outWriter := MakeOutStringWriter()
	go outWriter.Start(c, "aTag")
	c <- Entry{File: "file.txt", Message: "One"}
	c <- Entry{File: "file.txt", Message: "Two"}
	time.Sleep(100 * time.Millisecond)

	if outWriter.String() != "[aTag] [file.txt] One\n[aTag] [file.txt] Two\n" {
		t.Fail()
	}
}
