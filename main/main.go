package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"tail_folders/command"
	"tail_folders/logger"
	"tail_folders/tail"
	"tail_folders/watcher"
)

var (
	exitCode   = 0
	appVersion string
	rev        string
)

func main() {
	// exit code could depend on the process exit code
	defer func() {
		os.Exit(exitCode)
	}()

	// processing command arguments
	folderPathsPtr := flag.String("folders", ".", "Paths of the folders to watch for log files, separated by comma (,). IT SHOULD NOT BE NESTED")
	recursivePtr := flag.Bool("recursive", true, "Whether or not recursive folders should be watched")
	expressionTypePtr := flag.String("filter_by", "glob", "Expression type: Either 'glob' or 'regex'")
	filterPtr := flag.String("filter", "*.log", "Filter expression to apply on filenames")
	tagPtr := flag.String("tag", "", "Optional tag to use for each line")
	versionPtr := flag.Bool("version", false, "Print the version")

	flag.Usage = func() {
		fmt.Printf("%s - %s", os.Args[0], "Application that scans a list of folders (recursively by default) and tails any file that matches the filename filter\n\n")
		fmt.Printf("Usage of %s <options> [-- command args]:\n", os.Args[0])
		fmt.Println("")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionPtr {
		if appVersion == "" {
			appVersion = "development"
		}
		if rev == "" {
			rev = "untracked"
		}
		fmt.Printf("%s (Rev: %s)\n", appVersion, rev)
		os.Exit(0)
	}

	folderPathsStr := strings.TrimSpace(*folderPathsPtr)
	expressionTypeStr := strings.TrimSpace(*expressionTypePtr)
	filterStr := strings.TrimSpace(*filterPtr)
	tagStr := strings.TrimSpace(*tagPtr)

	// initialize loggers
	logFile := logger.CreateLogFile()
	logger.InitLogs(logFile, logFile, logFile, logFile)

	logger.Info.Println("Arguments in place:")
	logger.Info.Printf("- folders: %s", folderPathsStr)
	logger.Info.Printf("- recursive: %v", *recursivePtr)
	logger.Info.Printf("- filter_by: %s", expressionTypeStr)
	logger.Info.Printf("- filter: %s", filterStr)
	logger.Info.Printf("- tag: %s", strings.TrimSpace(tagStr))
	if flag.NArg() > 0 {
		logger.Info.Printf("- command: %v", flag.Args())
	}

	// run program
	run(folderPathsStr, expressionTypeStr, filterStr, tagStr, *recursivePtr, flag.Args())
}

func run(folderPathsStr, expressionTypeStr, filterStr, tagStr string, recursive bool, commandAndArguments []string) {
	// create filename filter
	filterFunc, err := createFilterFunc(expressionTypeStr, filterStr)
	if err != nil {
		log.Fatalf("Unrecognized filter_by value: %s", expressionTypeStr)
	}

	// init program
	stdoutChan := make(chan string)
	go tail.StdoutWriter(stdoutChan, tagStr)

	for _, folderPath := range strings.Split(folderPathsStr, ",") {
		rootFolderWatcher := watcher.MakeRootFolderWatcher(folderPath, stdoutChan, recursive, filterFunc)
		defer rootFolderWatcher.Close()
		rootFolderWatcher.Watch()
	}

	// if num arguments is greater than one, means that it is a command that should be started
	if len(commandAndArguments) > 0 {
		commandName := commandAndArguments[0]
		args := commandAndArguments[1:]
		logger.Info.Printf("Executing command '%s %s'...", commandName, strings.Join(args, " "))
		exitCode = command.ExecuteCommand(commandName, args)
	} else {
		// I need to block the program waiting for a signal to come for finishing this process
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		// block here until signal is received
		<-stop
	}
}

func createFilterFunc(expressionTypeStr string, filterStr string) (func(string) bool, error) {
	var filterFunc func(string) bool
	switch expressionTypeStr {
	case "glob":
		filterFunc = filterByGlob(filterStr)
	case "regex":
		filterFunc = filterByRegex(filterStr)
	default:
		return nil, fmt.Errorf("Unrecognized filter_by value: %s", expressionTypeStr)
	}
	return filterFunc, nil
}

func filterByGlob(globPattern string) func(string) bool {
	_, err := filepath.Match(globPattern, "text.txt")
	if err != nil {
		log.Fatalf("Globbing Expression '%s' is not right: %v", globPattern, err)
	}

	return func(filename string) bool {
		matched, _ := filepath.Match(globPattern, filename)
		return matched
	}
}

func filterByRegex(regexStr string) func(string) bool {
	regex := regexp.MustCompile(regexStr)
	return func(filename string) bool {
		return regex.MatchString(filename)
	}
}
