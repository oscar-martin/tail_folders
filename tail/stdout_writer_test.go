package tail

import (
	"testing"
	"time"
)

func TestStdoutWriterWithoutTag(t *testing.T) {
	c := make(chan string)
	outWriter := MakeOutStringWriter()
	go outWriter.Start(c, "")
	c <- "One\n"
	c <- "Two\n"
	time.Sleep(100 * time.Millisecond)

	if outWriter.String() != "One\nTwo\n" {
		t.Fail()
	}
}

func TestStdoutWriterWithTag(t *testing.T) {
	c := make(chan string)
	outWriter := MakeOutStringWriter()
	go outWriter.Start(c, "aTag")
	c <- "One\n"
	c <- "Two\n"
	time.Sleep(100 * time.Millisecond)

	if outWriter.String() != "[aTag] One\n[aTag] Two\n" {
		t.Fail()
	}
}
