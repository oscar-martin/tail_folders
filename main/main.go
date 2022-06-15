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

	"github.com/oscar-martin/tail_folders/command"
	"github.com/oscar-martin/tail_folders/logger"
	"github.com/oscar-martin/tail_folders/tail"
	"github.com/oscar-martin/tail_folders/watcher"
)

const (
	outputJson = "json"
	outputRaw  = "raw"
)

var (
	exitCode   = 0
	appVersion string
	rev        string
)

func main() {
	// p := profile.Start(profile.MemProfile, profile.ProfilePath("."), profile.NoShutdownHook)
	// p := profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.NoShutdownHook)
	// exit code could depend on the process exit code
	defer func() {
		os.Exit(exitCode)
	}()

	// processing command arguments
	folderPathsPtr := flag.String("folders", ".", "Paths of the folders to watch for log files, separated by comma (,). IT SHOULD NOT BE NESTED")
	recursivePtr := flag.Bool("recursive", true, "Whether or not recursive folders should be watched")
	expressionTypePtr := flag.String("filter_by", "glob", "Expression type: Either 'glob' or 'regex'")
	filterPtr := flag.String("filter", "*.log", "Filter expression to apply on filenames")
	contentFilterTypePtr := flag.String("content_filter_by", "no-filter", "Content filter type: Either 'include', 'exclude', 'regex' or 'no-filter'")
	contentFilterPtr := flag.String("content_filter", "", "Filter expression to apply on tailed lines")
	tagPtr := flag.String("tag", "", "Optional tag to use for each line")
	outputPtr := flag.String("output", "json", "Output type: Either 'raw' or 'json'")
	timeoutPtr := flag.Int("timeout", -1, "Time to wait till stop tailing when no activity is detected in a folder (seconds)")
	oldFilesPtr := flag.Int("discard-files-older-than", -1, "Discard tailing files not recently modified (seconds)")
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
	contentFilterTypeStr := strings.TrimSpace(*contentFilterTypePtr)
	contentFilterStr := strings.TrimSpace(*contentFilterPtr)
	tagStr := strings.TrimSpace(*tagPtr)
	outputStr := strings.TrimSpace(*outputPtr)
	timeout := *timeoutPtr
	oldFiles := *oldFilesPtr

	// initialize loggers
	logFile := logger.CreateLogFile()
	logger.InitLogs(logFile, logFile, logFile, logFile)

	logger.Info.Println("Arguments in place:")
	logger.Info.Printf("- folders: %s", folderPathsStr)
	logger.Info.Printf("- recursive: %v", *recursivePtr)
	logger.Info.Printf("- filter_by: %s", expressionTypeStr)
	logger.Info.Printf("- filter: %s", filterStr)
	logger.Info.Printf("- content_filter_by: %s", contentFilterTypeStr)
	logger.Info.Printf("- content_filter: %s", contentFilterStr)
	logger.Info.Printf("- tag: %s", tagStr)
	logger.Info.Printf("- output: %s", outputStr)
	logger.Info.Printf("- timeout: %d", timeout)
	logger.Info.Printf("- discard-files-older-than: %d", oldFiles)
	if flag.NArg() > 0 {
		logger.Info.Printf("- command: %v", flag.Args())
	}

	// create output func
	outputFunc, err := createEntryToStringFunc(outputStr)
	if err != nil {
		log.Fatal(err)
	}
	// run program
	outWriter := tail.MakeStdOutWriter(outputFunc)
	run(folderPathsStr, expressionTypeStr, filterStr, contentFilterTypeStr, contentFilterStr, tagStr, *recursivePtr, flag.Args(), outWriter, timeout, oldFiles)
	// p.Stop()
}

func run(
	folderPathsStr,
	expressionTypeStr,
	filterStr,
	contentFilterTypeStr,
	contentFilterStr,
	tagStr string,
	recursive bool,
	commandAndArguments []string,
	ow *tail.OutWriter,
	timeout,
	oldFiles int) {
	// create filename filter
	filterFunc, err := createFilterFunc(expressionTypeStr, filterStr)
	if err != nil {
		log.Fatal(err)
	}

	// create content filter
	contentFilterFunc, err := createContentFilterFunc(contentFilterTypeStr, contentFilterStr)
	if err != nil {
		log.Fatal(err)
	}

	// init program
	stdoutChan := make(chan tail.Entry)
	go ow.Start(stdoutChan, tagStr)

	for _, folderPath := range strings.Split(folderPathsStr, ",") {
		rootFolderWatcher := watcher.MakeRootFolderWatcher(folderPath, stdoutChan, recursive, filterFunc, contentFilterFunc, timeout, oldFiles)
		defer rootFolderWatcher.Close()
		err := rootFolderWatcher.Watch()
		if err != nil {
			log.Fatal(err)
		}
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

func createEntryToStringFunc(outputStr string) (func(tail.Entry, string) (string, error), error) {
	var outputFunc func(tail.Entry, string) (string, error)
	switch outputStr {
	case outputRaw:
		outputFunc = tail.EntryToRawString
	case outputJson:
		outputFunc = tail.EntryToJsonString
	default:
		return nil, fmt.Errorf("Unrecognized output value: %s", outputStr)
	}
	return outputFunc, nil
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

func createContentFilterFunc(filterTypeStr string, filterStr string) (func(string) bool, error) {
	var filterFunc func(string) bool
	switch filterTypeStr {
	case "exclude":
		filterFunc = contentFilterNotContain(filterStr)
	case "include":
		filterFunc = contentFilterContain(filterStr)
	case "regex":
		filterFunc = contentFilterByRegex(filterStr)
	case "no-filter":
		filterFunc = func(string) bool { return true }
	default:
		return nil, fmt.Errorf("Unrecognized content_filter_by value: %s", filterTypeStr)
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

func contentFilterContain(filter string) func(string) bool {
	return func(msg string) bool {
		return strings.Contains(msg, filter)
	}
}

func contentFilterNotContain(filter string) func(string) bool {
	return func(msg string) bool {
		return !strings.Contains(msg, filter)
	}
}

func contentFilterByRegex(regexStr string) func(string) bool {
	regex := regexp.MustCompile(regexStr)
	return func(msg string) bool {
		return regex.MatchString(msg)
	}
}
