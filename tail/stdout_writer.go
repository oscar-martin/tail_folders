package tail

import (
	"fmt"
)

func StdoutWriter(c <-chan string, tag string) {
	for {
		select {
		case logMsg := <-c:
			// actual write to stdout
			if tag == "" {
				fmt.Print(logMsg)
			} else {
				fmt.Printf("[%s] %s", tag, logMsg)
			}
		}
	}
}
