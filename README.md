# tail_folders

[![Build Status](https://travis-ci.org/oscar-martin/tail_folders.svg?branch=master)](https://travis-ci.org/oscar-martin/tail_folders)

This program scans within a list of folders passed in certain log (or any) files and tails them on current process' stdout. `tail_folders` listens to changes in the directory being monitored and when it finds a matching file, it will start tailing it. Files and folders (nested to one of the initial folders being tracked) can be created after `tail_folders` is started and it will automatically detect process them accordingly.

A use case for this tool is to create a docker image with this tool and to put a docker logging driver plugin on it. By running `tail_folders` as PID 1 in this container, its stdout/stderr will be sent to the desired log system. This way you can easily aggregate log files generated from other containers running on the same host by just creating volumes for folders where the logs are written and to share these volumes with the tail_folders container.

`tail_folders` also can start a process and it can tail a log file (or some) generated by the process. Note that `tail_folders` should be configured with the right parameters in order to scan the target folder where the process will create their log file/s. When `tail_folders` is launched in this way, it will end as soon as the process ends and the exit code of `tail_folders` will be the one from the process.

A use case for this could be to scope a program to be used in **OpenFaas**. There are times when you have a process that write garbage along with the actual output to stdout. Some log warnings, some legacy code that is difficult to change are examples of it. `tail_folders` can scope the program and will output from its stdout the content of a log file where the process will only write the actual information that needs to be sent.

Available parameters:

```shell
  -filter string
    	Filter expression to apply on filenames (default "*.log")
  -filter_by string
    	Expression type: Either 'glob' or 'regex'. Defaults to 'glob' (default "glob")
  -folders string
    	Paths of the folders to watch for log files, separated by comma (,). IT SHOULD NOT BE NESTED. Defaults to current directory (default ".")
  -recursive
    	Whether or not recursive folders should be watched (default true)
  -tag string
    	Optional tag to use for each line
  -output string
        Output type: Either 'raw' or 'json' (default "json")
```

`tail_folders` generates a log file that is found in `working_dir/.logdir/taillog.log`. **Note**: if `tail_folders` starts a new process, the stdout/stderr of that process will be written to `tail_folders`'s log.

JSON output follows this data structure:

```go
// Entry models a line read from a source file
type Entry struct {
	// Tag is user-provided setting for different tail_folders processes running
	// in a single host
	Tag string `json:"tag,omitempty"`
	// Hostname is the hostname where tail_folders is running
	Hostname string `json:"host,omitempty"`
	// Folders is a list of folder names where the source file is
	Folders []string `json:"dirs,omitempty"`
	// Filename is the base filename of the source file
	Filename string `json:"file,omitempty"`
	// File is the whole filepath. This field is for internal use only
	File string `json:"-"`
	// Message is the actual payload read from the source file
	Message string `json:"msg,omitempty"`
	// Timestamp is the time where the log is read
	Timestamp time.Time `json:"time,omitempty"`
}
```

So an example of a output line can be:

```raw
{"host":"MacBook-Pro.local","dirs":["tmp"],"file":"hola.log","msg":"aaaa","time":"2019-05-05T20:26:59.596488+02:00"}
```

## Sample of tailing a given set of folders

```shell
./tail_folders -tag=node1 -folders="./aa,./bb"
```

You can now create folder `./aa` and `./bb`. Then you can `touch ./aa/app_1.log` and `touch ./bb/app_2.log`.

Then open two new terminal sessions and execute `while true; do echo "aaaa" >> ./aa/app_1.log; sleep 1; done` and `while true; do echo "bbbbbbbb" >> ./bb/app_2.log; sleep 1; done` in each.

And the stdout of `tail_folders` should be similar to:

```raw
...
[node1] [bb/app_2.log] bbbbbbbb
[node1] [aa/app_1.log] aaaa
[node1] [aa/app_1.log] aaaa
[node1] [bb/app_2.log] bbbbbbbb
[node1] [aa/app_1.log] aaaa
...
```

**Do not forget to `CTRL-C` both `while true` processes and remove the files and folders.**

### Sample of tailing the log files generated by `./app.sh`

This sample assumes the process will write its log into current folder and the log file will have the .log extension. If it is not the case, please use the parameters to tweak the configuration.

```shell
./tail_folders -folders /tmp -recursive=false -output raw -- ./app.sh
[/tmp/test.log] aaaa
[/tmp/test.log] aaaa
[/tmp/test.log] aaaa
```
