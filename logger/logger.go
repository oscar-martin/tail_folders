package logger

import (
	"io"
	"log"
	"os"
	"path"
)

var (
	Info          *log.Logger
	Warning       *log.Logger
	Error         *log.Logger
	ProcessLog    *log.Logger
	logFolderName = "./.logdir"
)

func InitLogs(
	infoHandle io.Writer,
	warningHandle io.Writer,
	processHandle io.Writer,
	errorHandle io.Writer) {

	Info = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ProcessLog = log.New(processHandle, "PROC: ", log.Ldate|log.Ltime)
	Warning = log.New(warningHandle, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func CreateLogFile() *os.File {
	// create the log folder and file. It is inside a hidden folder for avoiding being tracked itself
	logFolder := path.Join(".", logFolderName)
	os.MkdirAll(logFolder, os.ModePerm)

	logFileName := path.Join(logFolder, "taillog.log")
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to open log file ", logFileName, ":", err)
	}
	return logFile
}
