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
)

type Options struct {
    Exec ExecOptions `json:"exec"`
    Instr InstrumentOptions `json:"instr"`
}

type ExecOptions struct {
    Path string `json:"path"`
    SharedMemSize string `json:"size"`
    NumProcesses int `json:"processes"`
    SchedFile string `json:"sched"`
    LogLevel string  `json:"loglevel"`
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

func copy_launch_script(dir string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    err = copy_file(cwd + "/exec_script.sh", dir + "/exec_script.sh")
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

func record(options ExecOptions) error {
    dir := get_directory_from_path(options.Path)
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
    err = build_target_program(dir)
    if err != nil {
        return err
    }
    set_environment(filepath.Base(dir))
    cmd, err := start_global_scheduler("record")
    if err != nil {
        return err
    }
    err = cmd.Wait()
    return err
}

func replay(options ExecOptions) error {
    dir := get_directory_from_path(options.Path)
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
    err = build_target_program(dir)
    if err != nil {
        return err
    }
    set_environment(filepath.Base(dir))
    cmd, err := start_global_scheduler("replay")
    if err != nil {
        return err
    }
    err = cmd.Wait()
    return err
}

func explore(options ExecOptions) error {
    dir := get_directory_from_path(options.Path)
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
    err = build_target_program(dir)
    if err != nil {
        return err
    }
    set_environment(filepath.Base(dir))
    cmd, err := start_global_scheduler("explore")
    if err != nil {
        return err
    }
    err = cmd.Wait()
    return err
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
    modePtr := flag.String("mode","","The action that needs to be performed : record, replay, explore, instrument")
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
    } else {
        fmt.Println("Invalid mode")
        os.Exit(1)
    }
}