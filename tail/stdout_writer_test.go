package tail

import "time"

func ExampleStdoutWriterWithoutTag() {
	c := make(chan string)
	go StdoutWriter(c, "")
	c <- "One\n"
	c <- "Two\n"
	time.Sleep(100 * time.Millisecond)
	// Output:
	// One
	// Two
}

func ExampleStdoutWriterWithTag() {
	c := make(chan string)
	go StdoutWriter(c, "aTag")
	c <- "One\n"
	c <- "Two\n"
	time.Sleep(100 * time.Millisecond)
	// Output:
	// [aTag] One
	// [aTag] Two
}
