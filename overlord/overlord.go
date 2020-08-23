package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/instrumenter"
	"io"
	"io/ioutil"
	"log"
    "net"
    "net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	l* log.Logger
)

//Options struct which configures the Dara run
//This stores the parsed config results
type Options struct {
	Exec  ExecOptions       `json:"exec"`
	Instr InstrumentOptions `json:"instr"`
	Bench BenchOptions      `json:"bench"`
}

//Custom build & run script options
type BuildOptions struct {
	BuildScript string `json:"build_path"`
	RunScript   string `json:"run_path"`
}

//Dara execution options
type ExecOptions struct {
	Path          string       `json:"path"`
	SharedMemSize string       `json:"size"`
	NumProcesses  int          `json:"processes"`
	SchedFile     string       `json:"sched"`
	LogLevel      string       `json:"loglevel"`
	Build         BuildOptions `json:"build"`
	PreloadReplay bool         `json:"fast_replay"`
	PropertyFile  string       `json:"property_file"`
	BlocksFile    string       `json:"blocks_file"`
	Strategy      string       `json:"strategy"`
	Microbenchmark bool        `json:"microbench"`
	Nanobenchmark bool         `json:"nanobench"`
	MaxDepth      int          `json:"maxdepth"`
	MaxRuns       int          `json:"maxruns"`
}

//Options specific for benchmarking
type BenchOptions struct {
	Outfile    string `json:"path"`
	Iterations int    `json:"iter"`
}

//Options specific for instrumentation
type InstrumentOptions struct {
	Dir  string `json:"dir"`
	File string `json:"file"`
	OutDir string `json:"outdir"`
	OutFile string `json:"outfile"`
	BlocksFile string `json:"blocks_file"`
}

type DaraRpcServer struct {
	Options ExecOptions
	logger *log.Logger
}

//Returns the directory from the path
func get_directory_from_path(path string) string {
	return filepath.Dir(path)
}

func write_blocks_file(filename string, blocks []string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, block := range blocks {
		_, err = f.WriteString(block + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

//Instruments a given file using Dinv's capture module
func instrument_file(filename string, outfile string) ([]string, error) {
	f, err := instrumenter.Annotate(filename)
	if err != nil {
		return []string{}, err
	}
	if outfile == "" {
		l.Println("Output file not provided; overwriting original file")
		outfile = filename
	}
	return f.GetBlockIDs(), f.WriteAnnotatedFile(outfile)
}

//Instruments all go files in a directory
func instrument_dir(directory string, outdir string, blocks_file string) error {
	if outdir == "" {
		l.Println("Output directory not provided; overwriting original directory")
		outdir = directory
	}
	var allBlocks []string
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
		}
		if strings.Contains(path, "vendor/") {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			outpath := strings.Replace(path, directory, outdir, -1)
			err = os.MkdirAll(filepath.Dir(outpath), 0777)
			if err != nil {
				return err
			}
			blocks, err := instrument_file(path, outpath)
			allBlocks = append(allBlocks, blocks...)
			return err
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	err = write_blocks_file(blocks_file, allBlocks)
	log.Println("Total number of blocks is", len(allBlocks))
	return err
}

//Sets the Program name as $PROGRAM for use in exec script
func set_environment(program string) {
	// Set the Program name here as PROGRAM
	os.Setenv("PROGRAM", program)
}

//Sets the $RUN_SCRIPT variable to the value provided in config file
func set_env_run_script(script string) {
	// Set the run script as RUN_SCRIPT
	os.Setenv("RUN_SCRIPT", script)
}

//Sets the $PROP_FILE variable to the value provided in config file
func set_env_property_file(filepath string) {
	// Set the property file as PROP_FILE
	os.Setenv("PROP_FILE", filepath)
}

//Sets the Fast replay option where the replay works from a loaded schedule
func set_fast_replay() {
	os.Setenv("FAST_REPLAY", "true")
}

//Sets the log level for the entire Dara run
func set_log_level(loglevel string) error {
	level := ""
	switch loglevel {
	case "DEBUG":
		level = "0"
	case "INFO":
		level = "1"
	case "WARN":
		level = "2"
	case "FATAL":
		level = "3"
	case "OFF":
		level = "4"
	default:
		return errors.New("Invalid log level specified in configuration file")
	}
	os.Setenv("DARA_LOG_LEVEL", level)
	return nil
}

//Sets the Dara mode environment variable
func set_dara_mode(mode string) {
	os.Setenv("DARA_MODE", mode)
}

//Sets Nanobenchmark enivornment variable
func set_nanobenchmark() {
	os.Setenv("NANOBENCH", "true")
}

//Set Microbenchmark environment variable
func set_microbenchmark() {
	os.Setenv("UBENCH", "true")
}

//Generic function for copying file from src to dst
func copy_file(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

//Installs the global scheduler
func install_global_scheduler() error {
	cmd := exec.Command("/usr/bin/dgo", "install", "github.com/DARA-Project/GoDist-Scheduler")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Failed to install global scheduler : ", err)
	}
	return err
}

//Launches the global scheduler and the run script to run the system
func launch_global_scheduler(mode string, numProcs int, sched_file string, strategy string, maxdepth int, maxruns int) (*exec.Cmd, error) {
	arg := "--" + mode + "=true --procs=" + strconv.Itoa(numProcs) + " --schedule=" + sched_file
	if strategy != "" {
		arg += " --strategy=" + strategy
	}
	if maxdepth != 0 {
		arg += " --maxdepth=" + strconv.Itoa(maxdepth)
	}
	if maxruns != 0 {
		arg += " --maxruns=" + strconv.Itoa(maxruns)
	}
	cmd := exec.Command("/bin/bash", "./exec_script.sh", arg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, err
}

//Starts running the go benchmark
func start_go_benchmark() (*exec.Cmd, error) {
	cmd := exec.Command("/bin/bash", "./bench_script.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, err
}

//Starts the global scheduler for this dara run
func start_global_scheduler(mode string, numProcs int, sched_file string, strategy string, maxdepth int, maxruns int) (*exec.Cmd, error) {
	err := install_global_scheduler()
	if err != nil {
		return nil, err
	}
	cmd, err := launch_global_scheduler(mode, numProcs, sched_file, strategy, maxdepth, maxruns)
	if err != nil {
		return nil, err
	}
	return cmd, err
}

//Setup the shared memory to be used by the global and local schedulers
func setup_shared_mem(size string, dir string) error {
	// Remove existing shared memory
	path := dir + "/DaraSharedMem"
	err := os.Remove(path)
	if err != nil {
		// Ignore if shared memory didn't exist
		err = nil
	}
	// Get shared memor from device 0
	outputFileArg := "of=" + path
	blockSize := "bs=" + size
	cmd := exec.Command("dd", "if=/dev/zero", outputFileArg, blockSize, "count=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}
	// Change permissions of shared memory
	err = os.Chmod(path, 0777)
	return err
}

//Execute the build script to build the program
func execute_build_script(script string, execution_dir string) error {
	cmd := exec.Command(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}
	l.Println("Finished building using build script")
	err = os.Chdir(execution_dir)
	return err
}

//Build the program executable using vanilla dgo
func build_target_program(dir string) error {
	err := os.Chdir(dir)
	if err != nil {
		return err
	}
	cmd := exec.Command("dgo", "build", "-v")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return err
}

//Build the program executable using vanilla go
func build_target_program_go(dir string) error {
	err := os.Chdir(dir)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-v")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return err
}

//Copies the execution script to the directory that contains the run script/executable
func copy_launch_script(dir string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	log.Println("Copying exec script from", cwd, " to ", dir)
	err = copy_file(cwd+"/exec_script.sh", dir+"/exec_script.sh")
	return err
}

//Copies the benchmarking script to the directory that contains the run script/executable
func copy_bench_script(dir string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = copy_file(cwd+"/bench_script.sh", dir+"/bench_script.sh")
	return err
}

//Initial handler for instrumentation mode
func instrument(options InstrumentOptions) error {
	if options.File == "" && options.Dir == "" {
		return errors.New("Instrument must have only one option(file or dir) selected.")
	}

	if options.BlocksFile == "" {
		return errors.New("Argument not provided for filename for the list of blocks")
	}

	if options.File != "" {
		blocks, err := instrument_file(options.File, options.OutFile)
		// Write blocks to the blocks file
		if err != nil {
			return err
		}
		err = write_blocks_file(options.BlocksFile,blocks)
		return err
	}

	if options.Dir != "" {
		return instrument_dir(options.Dir, options.OutDir, options.BlocksFile)
	}

	return nil
}

func (d * DaraRpcServer) killprogram() error {
	dir := get_directory_from_path(d.Options.Path)
    program := filepath.Base(dir)
    if d.Options.Build.RunScript == "" {
        cmd := exec.Command("pkill", program)
        err := cmd.Run()
        if err != nil {
			// This "error" means the program ended before we could kill it
            //d.logger.Println("Error while killing program", err)
            return err
        }
    } else {
        cmd := exec.Command("pkill", d.Options.Build.RunScript)
        err := cmd.Run()
        if err != nil {
			// This "error" means the program ended before we could kill it
            //d.logger.Println("Error while killing program", err)
            return err
        }
    }
    return nil
}

func (d * DaraRpcServer) KillExecution(unused_arg int, ack *bool) error {
    // Issue a kill command for killing the program under test
    err := d.killprogram()
    if err != nil {
        d.logger.Println("Failed to kill program")
        return err
    }
    *ack = true
    return nil
}

func (d * DaraRpcServer) RestartExecution(unused_arg int, ack * bool) error {
	f, err := os.Create("./explore_restart")
	if err != nil {
		d.logger.Println("Failed to finish exploration")
        return err
	}
	f.Close()
	return nil
}

func (d * DaraRpcServer) FinishExecution(unused_arg int, ack *bool) error {
    // Issue a finish command to the exec script somehow
	cwd, err := os.Getwd()
    if err != nil {
        d.logger.Println("Error while getting current directory", err)
        return err
    }
	d.logger.Println("Current Directory:", cwd)
    // Just create a file that is called explore_finish to signify end of exploration and the exec script can just stat if the file exists
    f, err := os.Create("./explore_finish")
    if err != nil {
        d.logger.Println("Failed to finish exploration")
        return err
    }
    f.Close()
    err = os.Chdir(cwd)
    if err != nil {
        d.logger.Println("Error while changing directory")
        return err
    }
    // Now we are ready to kill the program!
    err = d.killprogram()
    if err != nil {
        l.Println("[Overlord-RpcServer] Failed to kill program")
        return err
    }
    *ack = true
    return nil
}

func init_rpc_server(options ExecOptions) *DaraRpcServer{
    server := DaraRpcServer{options, log.New(os.Stdout, "[Overlord-RpcServer]", log.Lshortfile)}
    return &server
}

func start_rpc_server(options ExecOptions) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:45000")
	// TODO: Maybe these errors should really be fatal errors
    if err != nil {
        l.Println("[Overlord] Failed to resolve TCP address for RPC Server")
        return
    }
    inbound, err := net.ListenTCP("tcp", addr)
    if err != nil {
        l.Println("[Overlord] Failed to initialize inbound listener for RPC Server")
        return
    }

    server := init_rpc_server(options)
    rpc.Register(server)
    rpc.Accept(inbound)
}

//Sets up the environment and scripts for benchmarking go programs
func go_setup(options ExecOptions) error {
	dir := get_directory_from_path(options.Path)
	err := copy_bench_script(dir)
	if err != nil {
		return err
	}
	err = build_target_program(dir)
	if err != nil {
		return err
	}
	set_environment(filepath.Base(dir))
	return nil
}

//Sets up the environment and scripts for running Dara
func setup(options ExecOptions, mode string) error {
	dir := get_directory_from_path(options.Path)
	set_dara_mode(mode)
	err := set_log_level(options.LogLevel)
	if err != nil {
		return err
	}
	if options.Nanobenchmark {
		set_nanobenchmark()
	}
	if options.Microbenchmark {
		set_microbenchmark()
	}
	err = copy_launch_script(dir)
	if err != nil {
		return err
	}
	err = setup_shared_mem(options.SharedMemSize, dir)
	if err != nil {
		return err
	}
	build_script := options.Build.BuildScript
	if build_script == "" {
		err = build_target_program(dir)
		if err != nil {
			return err
		}
	} else {
		err = execute_build_script(build_script, dir)
		if err != nil {
			return err
		}
		set_env_run_script(options.Build.RunScript)
	}
	set_environment(filepath.Base(dir))
	set_env_property_file(options.PropertyFile)
	return nil
}

//Handler for recording executions
func record(options ExecOptions) error {
	err := setup(options, "record")
	if err != nil {
		return err
	}
	cmd, err := start_global_scheduler("record", options.NumProcesses, options.SchedFile, "", 0, 0)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	return err
}

//Handler for replaying executions
func replay(options ExecOptions) error {
	err := setup(options, "replay")
	if err != nil {
		return err
	}
	if options.PreloadReplay {
		set_fast_replay()
	}
	cmd, err := start_global_scheduler("replay", options.NumProcesses, options.SchedFile, "", 0, 0)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	return err
}

func post_exploration_cleanup(options ExecOptions) error {
	// Remove the explore_finish file which was used to terminate the exec_script
    return os.Remove("./explore_finish")
}

//Handler for exploring state space of a program
func explore(options ExecOptions) error {
	err := setup(options, "explore")
	if err != nil {
		return err
	}
    go start_rpc_server(options)
	cmd, err := start_global_scheduler("explore", options.NumProcesses, options.SchedFile, options.Strategy, options.MaxDepth, options.MaxRuns)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	err = post_exploration_cleanup(options)
	return err
}

//Handler for benchmarking between go and dgo
func bench(options ExecOptions, bOptions BenchOptions) error {
	NUM_ITERATIONS := bOptions.Iterations
	normal_vals := make([]float64, NUM_ITERATIONS)
	record_vals := make([]float64, NUM_ITERATIONS)
	replay_vals := make([]float64, NUM_ITERATIONS)
	fast_replay_vals := make([]float64, NUM_ITERATIONS)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = go_setup(options)
	if err != nil {
		return err
	}
	for i := 0; i < NUM_ITERATIONS; i++ {
		fmt.Println("Normal Iteration #", i)
		start := time.Now()
		cmd, err := start_go_benchmark()
		if err != nil {
			return err
		}
		err = cmd.Wait()
		normal_vals[i] = time.Since(start).Seconds()
		if err != nil {
			return err
		}
	}
	//os.Setenv("BENCH_RECORD", "true")
	for i := 0; i < NUM_ITERATIONS; i++ {
		// Reset working directory
		err = os.Chdir(cwd)
		if err != nil {
			return err
		}
		err = setup(options, "record")
		if err != nil {
			return err
		}
		fmt.Println("Record Iteration #", i)
		start := time.Now()
		cmd, err := start_global_scheduler("record", options.NumProcesses, options.SchedFile, "", 0, 0)
		if err != nil {
			return err
		}
		err = cmd.Wait()
		record_vals[i] = time.Since(start).Seconds()
		if err != nil {
			return err
		}
		//dat, err := ioutil.ReadFile("record.tmp")
		//if err != nil {
		//    return err
		//}
		//record_time, err := strconv.ParseFloat(strings.TrimSpace(string(dat)), 64)
		//if err != nil {
		//    return err
		//}
		//record_vals[i] = record_time
		if err != nil {
			return err
		}
	}
	//os.Unsetenv("BENCH_RECORD")
	for i := 0; i < NUM_ITERATIONS; i++ {
		// Reset working directory
		err = os.Chdir(cwd)
		if err != nil {
			return err
		}
		err = setup(options, "replay")
		if err != nil {
			return err
		}
		fmt.Println("Replay Iteration #", i)
		start := time.Now()
		cmd, err := start_global_scheduler("replay", options.NumProcesses, options.SchedFile, "", 0, 0)
		if err != nil {
			return err
		}
		err = cmd.Wait()
		replay_vals[i] = time.Since(start).Seconds()
		if err != nil {
			return err
		}
	}
	if options.PreloadReplay {
		for i := 0; i < NUM_ITERATIONS; i++ {
			err = os.Chdir(cwd)
			if err != nil {
				return err
			}
			err = setup(options, "replay")
			if err != nil {
				return err
			}
			set_fast_replay()
			fmt.Println("Fast Replay Iteration #", i)
			start := time.Now()
			cmd, err := start_global_scheduler("replay", options.NumProcesses, options.SchedFile, "", 0, 0)
			if err != nil {
				return err
			}
			err = cmd.Wait()
			fast_replay_vals[i] = time.Since(start).Seconds()
			if err != nil {
				return err
			}
		}
	}
	f, err := os.Create(bOptions.Outfile)
	if err != nil {
		return err
	}
	defer f.Close()
	header_string := "Normal,Record,Replay"
	if options.PreloadReplay {
		header_string += ",Fast_Replay"
	}
	_, err = f.WriteString(header_string + "\n")
	if err != nil {
		return err
	}
	for i := 0; i < NUM_ITERATIONS; i++ {
		val0 := normal_vals[i]
		val1 := record_vals[i]
		val2 := replay_vals[i]
		s := fmt.Sprintf("%f,%f,%f", val0, val1, val2)
		if options.PreloadReplay {
			val3 := fast_replay_vals[i]
			s = fmt.Sprintf("%s,%f", s, val3)
		}
		_, err = f.WriteString(s + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

//Parse the config file provided by command line
func parse_options(optionsFile string) (options Options, err error) {
	file, err := os.Open(optionsFile)
	if err != nil {
		return options, err
	}
	defer file.Close()
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return options, err
	}
	json.Unmarshal(bytes, &options)
	return options, nil
}

func main() {
	modePtr := flag.String("mode", "", "The action that needs to be performed : record, replay, explore, instrument, benchmark")
	filePtr := flag.String("optFile", "", "json file containing the configuration options")

	flag.Parse()

	l = log.New(os.Stdout, "[Overlord]", log.Lshortfile)

	if *modePtr == "" || *filePtr == "" {
		l.Fatal("Usage : go run overlord.go -mode=[record,replay,explore,instrument] -optFile=<path_to_options_file>")
	}

	options, err := parse_options(*filePtr)
	if err != nil {
		l.Fatal(err)
	}

	if *modePtr == "instrument" {
		err := instrument(options.Instr)
		if err != nil {
			l.Fatal("Failed to instrument file : ", err)
		}
	} else if *modePtr == "record" {
		err := record(options.Exec)
		if err != nil {
			l.Fatal("Failed to record execution : ", err)
		}
	} else if *modePtr == "replay" {
		err := replay(options.Exec)
		if err != nil {
			l.Fatal("Failed to replay execution : ", err)
		}
	} else if *modePtr == "explore" {
		err := explore(options.Exec)
		if err != nil {
			l.Fatal("Failed to explore : ", err)
		}
	} else if *modePtr == "bench" {
		err := bench(options.Exec, options.Bench)
		if err != nil {
			l.Fatal("Failed to bench : ", err)
		}
	} else {
		l.Fatal("Invalid mode")
	}
}
