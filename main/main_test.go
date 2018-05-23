package main

import (
	"fmt"
	"os"
	"syscall"
	_ "testing"
	"time"
)

func sendInterruptToMyselfAfter(d time.Duration) {
	time.AfterFunc(d, func() {
		myPid := os.Getpid()
		process, err := os.FindProcess(myPid)
		if err != nil {
			panic(err)
		}

		process.Signal(syscall.SIGINT)
	})
}

func runMain(mainFunc func()) <-chan struct{} {
	syncChan := make(chan struct{})
	go func() {
		mainFunc()
		syncChan <- struct{}{}
	}()
	time.Sleep(100 * time.Millisecond)
	return syncChan
}

func isError(err error) bool {
	if err != nil {
		fmt.Println(err.Error())
	}

	return (err != nil)
}

func createFile(path string) *os.File {
	// detect if file exists
	_, err := os.Stat(path)

	// create file if not exists
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		return file
	}
	panic(fmt.Sprintf("File %s already exists", path))
}

func writeInFile(file *os.File, line string) {
	if _, err := file.Write([]byte(line)); err != nil {
		panic(err)
	}
}

func closeFile(file *os.File) {
	if err := file.Close(); err != nil {
		panic(err)
	}
}

// Write into a log file. The output should see what is written
func ExampleRun_tailOnSingleFile() {
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "glob", "file*.log", "", false, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)

	// Output:
	// [/tmp/file1.log] temporary file's content
}

// Write into a log file. The output should see what is written with a tag
func ExampleRun_tailOnSingleFileWithTag() {
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "glob", "file*.log", "aTag", false, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)

	// Output:
	// [aTag] [/tmp/file1.log] temporary file's content
}

// Write into a txt file and a log file. The output should only see what is
// written into the log file. Filter based on glob pattern
func ExampleRun_tailOnSingleFileWithGlobFilterExecution() {
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	pathTxt := "/tmp/file1.txt"
	tmpfileTxt := createFile(pathTxt)
	defer os.Remove(tmpfileTxt.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "glob", "file*.log", "", false, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")
	writeInFile(tmpfileTxt, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)
	closeFile(tmpfileTxt)

	// Output:
	// [/tmp/file1.log] temporary file's content
}

// Write into two log files, one of them in a nested folder. The output should only see what is
// written into the log file from the non-nested folder. Filter based on glob pattern
func ExampleRun_tailOnNonRecursiveSingleFileWithGlobFilterExecution() {
	os.MkdirAll("/tmp/tail_folder_test", os.ModePerm)
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	pathInnerFolder := "/tmp/tail_folder_test/file1.log"
	tmpfileInner := createFile(pathInnerFolder)
	defer os.Remove(tmpfileInner.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "glob", "file*.log", "", false, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")
	writeInFile(tmpfileInner, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)
	closeFile(tmpfileInner)

	// Output:
	// [/tmp/file1.log] temporary file's content
}

// Write into two log files, one of them in a nested folder. The output should only see what is
// written into the log file from the both folders. Filter based on glob pattern
func ExampleRun_tailOnRecursiveSingleFileWithGlobFilterExecution() {
	os.MkdirAll("/tmp/tail_folder_test", os.ModePerm)
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	pathInnerFolder := "/tmp/tail_folder_test/file1.log"
	tmpfileInner := createFile(pathInnerFolder)
	defer os.Remove(tmpfileInner.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "glob", "file*.log", "", true, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")
	time.Sleep(10 * time.Millisecond)
	writeInFile(tmpfileInner, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)
	closeFile(tmpfileInner)

	// Output:
	// [/tmp/file1.log] temporary file's content
	// [/tmp/tail_folder_test/file1.log] temporary file's content
}

// Write into a txt file and a log file. The output should only see what is
// written into the log file. Filter based on regex pattern
func ExampleRun_tailOnSingleFileWithRegexFilterExecution() {
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	pathTxt := "/tmp/file1.txt"
	tmpfileTxt := createFile(pathTxt)
	defer os.Remove(tmpfileTxt.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "regex", "file.\\.[gol]{3}", "", false, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")
	writeInFile(tmpfileTxt, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)
	closeFile(tmpfileTxt)

	// Output:
	// [/tmp/file1.log] temporary file's content
}

// Write into a txt file and a log file. The output should only see what is
// written into the log file. Filter based on glob pattern
func ExampleRun_tailOnTwoFiles() {
	path := "/tmp/file1.log"
	tmpfile := createFile(path)
	defer os.Remove(tmpfile.Name()) // clean up

	pathTxt := "/tmp/file1.txt"
	tmpfileTxt := createFile(pathTxt)
	defer os.Remove(tmpfileTxt.Name()) // clean up

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	exit := runMain(func() {
		run("/tmp", "glob", "file1.*", "", false, make([]string, 0))
	})

	writeInFile(tmpfile, "temporary file's content\n")
	time.Sleep(10 * time.Millisecond)
	writeInFile(tmpfileTxt, "temporary file's content\n")

	<-exit

	closeFile(tmpfile)
	closeFile(tmpfileTxt)

	// Output:
	// [/tmp/file1.log] temporary file's content
	// [/tmp/file1.txt] temporary file's content
}

//func ExampleCommandToScope() {
//	run("/tmp", "glob", "hola.log", "", false, []string{"../app.sh"})
//
//	// Output:
//	// [/tmp/hola.log] aaaa
//	// [/tmp/hola.log] aaaa
//	// [/tmp/hola.log] aaaa
//}