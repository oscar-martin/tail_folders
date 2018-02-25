# tail_folders 

[![Build Status](https://travis-ci.org/oscar-martin/tail_folders.svg?branch=master)](https://travis-ci.org/oscar-martin/tail_folders)

This program scans within a list of folders passed in certain log files and tails them on current process' stdout. 

A use case for this tool is to create a docker image with this tool and to put a logging driver plugin on it. By running `tail_folders` as PID 1 in this container, its stdout/stderr will be sent to the desired log system. This way you can easily aggregate log files generated from other containers running on the same host by just creating volumes for folders where the logs are written and to share these volumes with the tail_folders container.

Available parameters:

```
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
```

Sample of usage:

```
./tail_folders -tag=node1 -folders="./aa,./bb"
```

You can now create folder `./aa` and `./bb`. Then you can `touch ./aa/app_1.log` and `touch ./bb/app_2.log`.

Then open two new terminal sessions and execute `while true; do echo "aaaa" >> ./aa/app_1.log; sleep 1; done` and `while true; do echo "bbbbbbbb" >> ./bb/app_2.log; sleep 1; done` in each.

And the stdout of `tail_folders` should be similar to:

```
...
[node1] [bb/app_2.log] bbbbbbbb
[node1] [aa/app_1.log] aaaa
[node1] [aa/app_1.log] aaaa
[node1] [bb/app_2.log] bbbbbbbb
[node1] [aa/app_1.log] aaaa
...
```

**Do not forget to `CTRL-C` both `while true` processes and remove the files and folders.**
