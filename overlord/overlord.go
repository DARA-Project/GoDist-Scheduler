package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "bitbucket.org/bestchai/dinv/capture"
    "time"
    "log"
//    "strconv"
//    "strings"
)

type Options struct {
    Exec ExecOptions `json:"exec"`
    Instr InstrumentOptions `json:"instr"`
    Bench BenchOptions `json:"bench"`
}

type BuildOptions struct {
    BuildScript string `json:"build_path"`
    RunScript string `json:"run_path"`
}

type ExecOptions struct {
    Path string `json:"path"`
    SharedMemSize string `json:"size"`
    NumProcesses int `json:"processes"`
    SchedFile string `json:"sched"`
    LogLevel string  `json:"loglevel"`
    Build BuildOptions `json:"build"`
}

type BenchOptions struct {
    Outfile string `json:"path"`
    Iterations int `json:"iter"`
}

type InstrumentOptions struct {
    Dir string `json:"dir"`
    File string `json:"file"`
}

func get_directory_from_path(path string) string {
    return filepath.Dir(path)
}

func write_instrumented_file(filename string, source_code string) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    file.WriteString(source_code)
    return nil
}

func instrument_file(filename string) error {
    options := make(map[string]string)
    options["file"] = filename
    output := capture.InsturmentComm(options)
    new_source := output[filename]
    return write_instrumented_file(filename, new_source)
}

func instrument_dir(directory string) error {
    err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                fmt.Println("Prevent panic by handling failure accessing a path %q: %v\n", path, err)
                return err
            }
            if !info.IsDir() && filepath.Ext(path) == ".go" {
                err = instrument_file(path)
                return err
            }
            return nil
    })
    return err
}

func set_environment(program string) {
    // Set the Program name here as PROGRAM
    os.Setenv("PROGRAM", program)
}

func set_env_run_script(script string) {
    // Set the run script as RUN_SCRIPT
    os.Setenv("RUN_SCRIPT", script)
}

func set_log_level(loglevel string) error {
    level := ""
    switch loglevel {
        case "DEBUG" :
            level = "0"
        case "INFO" :
            level = "1"
        case "WARN" :
            level = "2"
        case "FATAL" :
            level = "3"
        case "OFF" :
            level = "4"
        default:
            return errors.New("Invalid log level specified in configuration file")
    }
    os.Setenv("DARA_LOG_LEVEL", level)
    return nil
}

func set_dara_mode(mode string) {
    os.Setenv("DARA_MODE", mode)
}

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

func launch_global_scheduler(mode string) (*exec.Cmd, error) {
    arg := "--" + mode + "=true"
    cmd := exec.Command("/bin/bash", "./exec_script.sh", arg)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Start()
    return cmd, err
}

func start_go_benchmark() (*exec.Cmd, error) {
    cmd := exec.Command("/bin/bash", "./bench_script.sh")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Start()
    return cmd, err
}

func start_global_scheduler(mode string) (*exec.Cmd, error) {
    err := install_global_scheduler()
    if err != nil {
        return nil, err
    }
    cmd, err := launch_global_scheduler(mode)
    if err != nil {
        return nil, err
    }
    return cmd, err
}

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

func execute_build_script(script string, execution_dir string) error {
    cmd := exec.Command(script)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Run()
    if err != nil {
        return err
    }
    log.Println("[Overlord]Finished building using build script")
    err = os.Chdir(execution_dir)
    return err
}

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

func copy_launch_script(dir string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    err = copy_file(cwd + "/exec_script.sh", dir + "/exec_script.sh")
    return err
}

func copy_bench_script(dir string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    err = copy_file(cwd + "/bench_script.sh", dir + "/bench_script.sh")
    return err
}

func instrument(options InstrumentOptions) error {
    if options.File == "" && options.Dir == "" {
        return errors.New("Instrument must have only one option(file or dir) selected.")
    }

    if options.File != "" {
        return instrument_file(options.File)
    }

    if options.Dir != "" {
        return instrument_dir(options.Dir)
    }

    return nil
}

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

func setup(options ExecOptions, mode string) error {
    dir := get_directory_from_path(options.Path)
    set_dara_mode(mode)
    err := set_log_level(options.LogLevel)
    if err != nil {
        return err
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
        err = execute_build_script(build_script,dir)
        if err != nil {
            return err
        }
        set_env_run_script(options.Build.RunScript)
    }
    set_environment(filepath.Base(dir))
    return nil
}

func record(options ExecOptions) error {
    err := setup(options, "record")
    if err != nil {
        return err
    }
    cmd, err := start_global_scheduler("record")
    if err != nil {
        return err
    }
    err = cmd.Wait()
    return err
}

func replay(options ExecOptions) error {
    err := setup(options, "replay")
    if err != nil {
        return err
    }
    cmd, err := start_global_scheduler("replay")
    if err != nil {
        return err
    }
    err = cmd.Wait()
    return err
}

func explore(options ExecOptions) error {
    err := setup(options, "explore")
    if err != nil {
        return err
    }
    cmd, err := start_global_scheduler("explore")
    if err != nil {
        return err
    }
    err = cmd.Wait()
    return err
}

func bench(options ExecOptions, bOptions BenchOptions) error {
    NUM_ITERATIONS := bOptions.Iterations
    normal_vals := make([]float64, NUM_ITERATIONS)
    record_vals := make([]float64, NUM_ITERATIONS)
    replay_vals := make([]float64, NUM_ITERATIONS)
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    err = go_setup(options)
    if err != nil {
        return err
    }
    for i := 0; i < NUM_ITERATIONS; i++ {
        fmt.Println("Normal Iteration #",i)
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
        fmt.Println("Record Iteration #",i)
        start := time.Now()
        cmd, err := start_global_scheduler("record")
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
        fmt.Println("Replay Iteration #",i)
        start := time.Now()
        cmd, err := start_global_scheduler("replay")
        if err != nil {
            return err
        }
        err = cmd.Wait()
        replay_vals[i] = time.Since(start).Seconds()
        if err != nil {
            return err
        }
    }
    f, err := os.Create(bOptions.Outfile)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = f.WriteString("Normal,Record,Replay\n")
    if err != nil {
        return err
    }
    for i:=0; i < NUM_ITERATIONS; i++ {
        val0 := normal_vals[i]
        val1 := record_vals[i]
        val2 := replay_vals[i]
        s := fmt.Sprintf("%f,%f,%f\n",val0, val1, val2)
        _, err = f.WriteString(s)
        if err != nil {
            return err
        }
    }
    return nil
}

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
    modePtr := flag.String("mode","","The action that needs to be performed : record, replay, explore, instrument, benchmark")
    filePtr := flag.String("optFile", "", "json file containing the configuration options")

    flag.Parse()

    if *modePtr == "" || *filePtr == "" {
        fmt.Println("Usage : go run overlord.go -mode=[record,replay,explore,instrument] -optFile=<path_to_options_file>")
        os.Exit(1)
    }

    options, err := parse_options(*filePtr)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    if *modePtr == "instrument" {
        err := instrument(options.Instr)
        if err != nil {
            fmt.Println("Failed to instrument file : ", err)
            os.Exit(1)
        }
    } else if *modePtr == "record" {
        err := record(options.Exec)
        if err != nil {
            fmt.Println("Failed to record execution : ", err)
            os.Exit(1)
        }
    } else if *modePtr == "replay" {
        err := replay(options.Exec)
        if err != nil {
            fmt.Println("Failed to replay execution : ", err)
            os.Exit(1)
        }
    } else if *modePtr == "explore" {
        err := explore(options.Exec)
        if err != nil {
            fmt.Println("Failed to explore : ", err)
            os.Exit(1)
        }
    } else if *modePtr == "bench" {
        err := bench(options.Exec, options.Bench)
        if err != nil {
            fmt.Println("Failed to bench : ",err)
            os.Exit(1)
        }
    } else {
        fmt.Println("Invalid mode")
        os.Exit(1)
    }
}
