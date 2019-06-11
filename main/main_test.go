package main

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/oscar-martin/tail_folders/tail"
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

func createFile(path string) (*os.File, func()) {
	// detect if file exists
	_, err := os.Stat(path)

	// create file if not exists
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		return file, func() {
			closeFile(file)
			os.Remove(file.Name())
		}
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	return file, func() {
		closeFile(file)
		os.Remove(file.Name())
	}

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
func TestTailOnSingleFile(t *testing.T) {
	path := "./file1.log"
	tmpfile, closeFunc := createFile(path)

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "glob", "file*.log", "", false, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc()

	if outWriter.String() != "[file1.log] temporary file's content\n" {
		t.Fail()
	}
}

// Write into a log file. The output should see what is written with a tag
func TestTailOnSingleFileWithTag(t *testing.T) {
	path := "./file2.log"
	tmpfile, closeFunc := createFile(path)

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "glob", "file*.log", "aTag", false, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc()

	wanted := "[aTag] [file2.log] temporary file's content\n"
	if outWriter.String() != wanted {
		t.Errorf("Found %s; wanted %s", outWriter.String(), wanted)
	}
}

// Write into a txt file and a log file. The output should only see what is
// written into the log file. Filter based on glob pattern
func TestTailOnSingleFileWithGlobFilterExecution(t *testing.T) {
	path := "./file3.log"
	tmpfile, closeFunc1 := createFile(path)

	pathTxt := "./file3.txt"
	tmpfileTxt, closeFunc2 := createFile(pathTxt)

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "glob", "file*.log", "", false, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	writeInFile(tmpfileTxt, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc1()
	defer closeFunc2()

	wanted := "[file3.log] temporary file's content\n"
	if outWriter.String() != wanted {
		t.Errorf("Found %s; wanted %s", outWriter.String(), wanted)
	}
}

// Write into two log files, one of them in a nested folder. The output should only see what is
// written into the log file from the non-nested folder. Filter based on glob pattern
func TestTailOnNonRecursiveSingleFileWithGlobFilterExecution(t *testing.T) {
	os.MkdirAll("./tail_folder_test", os.ModePerm)
	path := "./file4.log"
	tmpfile, closeFunc1 := createFile(path)

	pathInnerFolder := "./tail_folder_test/file4.log"
	tmpfileInner, closeFunc2 := createFile(pathInnerFolder)

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "glob", "file*.log", "", false, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	writeInFile(tmpfileInner, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc1()
	defer closeFunc2()

	if outWriter.String() != "[file4.log] temporary file's content\n" {
		t.Fail()
	}
}

// Write into two log files, one of them in a nested folder. The output should only see what is
// written into the log file from the both folders. Filter based on glob pattern
func TestTailOnRecursiveSingleFileWithGlobFilterExecution(t *testing.T) {
	folderName := "./tail_folder_test"
	os.MkdirAll(folderName, os.ModePerm)
	path := "./file5.log"
	tmpfile, closeFunc1 := createFile(path)

	pathInnerFolder := fmt.Sprintf("%s/file5.log", folderName)
	tmpfileInner, closeFunc2 := createFile(pathInnerFolder)

	sendInterruptToMyselfAfter(300 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "glob", "file*.log", "", true, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)
	writeInFile(tmpfileInner, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc1()
	defer closeFunc2()
	defer os.RemoveAll(folderName)

	wanted := "[file5.log] temporary file's content\n[tail_folder_test/file5.log] temporary file's content\n"
	if outWriter.String() != wanted {
		t.Errorf("Found: %s; wanted: %s", outWriter.String(), wanted)
	}
}

// Write into a txt file and a log file. The output should only see what is
// written into the log file. Filter based on regex pattern
func TestTailOnSingleFileWithRegexFilterExecution(t *testing.T) {
	path := "./file6.log"
	tmpfile, closeFunc1 := createFile(path)

	pathTxt := "./file6.txt"
	tmpfileTxt, closeFunc2 := createFile(pathTxt)

	sendInterruptToMyselfAfter(200 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "regex", "file.\\.[gol]{3}", "", false, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	writeInFile(tmpfileTxt, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc1()
	defer closeFunc2()

	if outWriter.String() != "[file6.log] temporary file's content\n" {
		t.Fail()
	}
}

// Write into a txt file and a log file. The output should only see what is
// written into the log file. Filter based on glob pattern
func TestTailOnTwoFiles(t *testing.T) {
	path := "./file7.log"
	tmpfile, closeFunc1 := createFile(path)

	pathTxt := "./file7.txt"
	tmpfileTxt, closeFunc2 := createFile(pathTxt)

	sendInterruptToMyselfAfter(300 * time.Millisecond)

	outWriter := tail.MakeOutStringWriter()
	exit := runMain(func() {
		run(".", "glob", "file7.*", "", false, make([]string, 0), outWriter, -1, -1)
	})

	writeInFile(tmpfile, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)
	writeInFile(tmpfileTxt, "temporary file's content\n")
	time.Sleep(100 * time.Millisecond)

	<-exit

	defer closeFunc1()
	defer closeFunc2()

	wanted := "[file7.log] temporary file's content\n[file7.txt] temporary file's content\n"
	if outWriter.String() != wanted {
		t.Errorf("Found: %s; wanted: %s", outWriter.String(), wanted)
	}
}

//func ExampleCommandToScope() {
//	run("/tmp", "glob", "hola.log", "", false, []string{"../app.sh"})
//
//	// Output:
//	// [/tmp/hola.log] aaaa
//	// [/tmp/hola.log] aaaa
//	// [/tmp/hola.log] aaaa
//}
