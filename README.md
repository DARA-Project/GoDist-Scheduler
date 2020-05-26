# GoDist-Scheduler
![scheduler](https://i.stack.imgur.com/bzDfr.png)

GoDist-Scheduler is a global scheduler for model checking distributed systems. The GoDist-Scheduler communicates to 
insturmented version of the Golang runtime via shared memory. The insturmented runtime can be found 
[here](https://github.com/DARA-Project/GoDist).

## Getting Started

To install GoDist-Scheduler you must have a working installation of [Go](https://golang.org/doc/install).
Second [GoDist](https://github.com/DARA-Project/GoDist) (insturmented go runtime) must be installed to `/usr/bin/dgo`
in future releases this location with be configurable.

### Environment Variables

Go packaging relies heavily on the GOPATH environment variable. Make sure that the GOPATH environment variable is set before moving on.
You can check if GOPATH is set by executing the following command:

```
    > echo $GOPATH # Output should be the path to which GOPATH is set
```

The bin folder of the GOPATH folder must also be a part of the PATH variable.
You can append it to the PATH variable as follows:

```
    > export PATH=$PATH:$GOPATH/bin
```

Ensure that the modified go runtime (dgo) is setup correctly

```
    > which dgo  # Output should be /usr/bin/dgo
```

## Running Dara

### Configuration File

Running the concrete model checker requires the use of a configuration file.

NOTE: This is not the final version of the configuration file as it will be modified
to support complicated distributed systems that have their own specific build and run scripts as well
as include configurable options for exploration strategies and depth.

```
    {
        "exec" : {
            "path" : "/path/to/examples/SimpleFileRead/file_read.go",
            "size" : "400M",
            "processes" : 1,
            "sched" : "file_read.json",
            "loglevel" : "INFO",
            "property_file" : "/path/to/property/file.prop"
            "build" : {
                "build_path" : "../examples/ServerClient/build.sh",
                "run_path" : "run.sh"
            }
        },
        "instr" : {
            "dir" : "",
            "file" : "../examples/SimpleInstrument/file_read.go"
        }
    }
```

The exec block in the configuration file refers to the execution options of the system.
The details of every field for all the execution options is presented as follows:

+ `path` : The full path to the file which contains the main function the system
+ `size` : The size of the shared memory to be used by the global-scheduler and the local go runtimes.
+ `processes` : The number of processes that need to be launched that will execute the main function
+ `sched` : The name of the schedule file that will be used to write to/read from for doing record/replay or exploration.
+ `loglevel` : The level of the logging that needs to be printed out to console. Supported levels are : DEBUG, INFO, WARN, FATAL
+ `property_file` : The path to the property file that contains the properties!
+ `build`: This is to allow for complicated build and run scripts.
+ `build_path`: Path to build script
+ `run_path`: Path to run script

The instr block in the configuration file refers to the instrumentation options of the system.
Currently either a single file can be instrumented or the whole directory can be instrumented.
The details of each field are as follows:

+ `dir` : The full path to the directory to be instrumented
+ `file` : The full path to the file to be instrumented

### Property File

A property file should only have function definitions
and comment for the function. The definition corresponds
to the property to be checked. The comment provides
the meta information: Property name and full qualified
path of each variable in the source package. The full
qualified path is used for data collection 

Caveat: The variables used in the property must start
with an uppercase letter

Example of a property:

```
//Equality
//main.a
//main.b
func equality(A int, B int) {
    return A == B
}
```

For a more concrete example, please look at this [example](examples/SharedIntegerNoLocksProperty).

### Record

To record a schedule, use the overlord program provided as part of the GoDist-Scheduler.
The steps for recording the schedule using the overlord is as follows:

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/overlord
    > go run overlord.go -mode=record -optFile=system_config.json
```

### Replay

To replay a schedule, use the overlord program provided as part of the GoDist-Scheduler.
The steps for replaying the schedule using the overlord is as follows:

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/overlord
    > go run overlord.go -mode=replay -optFile=system_config.json
```

### Explorer (experimental)

To explore different interleavings for a system, use the overlord program provided as part of the GoDist-Scheduler.
The steps for exploration using the overlord is as follows:

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/overlord
    > go run overlord.go -mode=explore -optFile=system_config.json
```

### Instrument

To instrument a system so that it reports coverage information, use the overlord program as follows:

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/overlord
    > go run overlord.go -mode=instrument -optFile=system_config.json
```

## Auxiliary Tools

Understanding the bug traces can be hard. Thus, to aid in the understanding of the debug traces,
we make use of existing tools like ShiViz and provide some of our own tools.

### Schedule Info Tool

The Schedule Info tool prints out meta information about a schedule. Currently, it prints
out the number of events in the schedule as well as breakdown of events by the type of the event.

To run the info tool, execute the following steps

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/tools
    > go run schedule_info.go <schedule_filename>
```

Sample output from running the schedule info is shown below

![info](img/info.png?raw=true)

### ShiViz Converter Tool

The ShiViz converter tool converts a recorded or replayed schedule into a ShiViz-compatible
look which can be used to visualize the bug trace using the [online Shiviz tool](https://bestchai.bitbucket.io/shiviz/).

To run the converter tool, execute the following steps

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/tools
    > go run shiviz_converter.go <schedule_filename> <shiviz_filename>
```

A snapshot of the ShiViz visualization of the generated log from the recorded schedule
is shown below:

![shiviz](img/shiviz.png?raw=true)


### Variable Info Tool

Specifying properties requies using the full name scope name of a variable.
We wrote a tool that helps you extract all the user-defined variables
in your code.

```
    > cd $GOPATH/src/github.com/DARA-Project/GoDist-Scheduler/tools
    > go run variable_info.go <path/to/binary> 
``` 
