package tail

import "time"

func ExampleStdoutWriter_withoutTag() {
	c := make(chan string)
	go StdoutWriter(c, "")
	c <- "One\n"
	c <- "Two\n"
	time.Sleep(100 * time.Millisecond)
	// Output:
	// One
	// Two
}

func ExampleStdoutWriter_withTag() {
	c := make(chan string)
	go StdoutWriter(c, "aTag")
	c <- "One\n"
	c <- "Two\n"
	time.Sleep(100 * time.Millisecond)
	// Output:
	// [aTag] One
	// [aTag] Two
}
