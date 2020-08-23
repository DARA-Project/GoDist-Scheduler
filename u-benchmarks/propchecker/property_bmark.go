package main

import (
    "github.com/DARA-Project/GoDist-Scheduler/propchecker"
    "os"
    "log"
    "math/rand"
    "math"
    "fmt"
    "time"
)

func main() {
    files := [4]string{"./sample1.prop", "./sample2.prop", "./sample5.prop", "./sample10.prop"}
    num_props := [4]string{"1", "2", "5", "10"}
    var build_times []float64
    var load_check_times []float64
    var check_times []float64
    var check_time_sds []float64
    for _, filename := range files {
        start := time.Now()
        checker, err := propchecker.NewChecker(filename)
        elapsed := time.Since(start)
        build_times = append(build_times, elapsed.Seconds())
        log.Printf("Parsing and Building took %s", elapsed)
        if err != nil {
            log.Fatal(err)
        }

        context := make(map[string]interface{})
        // The choice of numbers really doesn't matter in a benchmarking context
        // since every property is going to be executed anyways.
        context["main.a"] = 15
        context["main.b"] = 20
        context["main.c"] = 0
        context["main.d"] = 10

        start = time.Now()
        result, failures, err := checker.Check(context)
        elapsed = time.Since(start)
        log.Printf("Checking Properties took %s\n", elapsed)
        load_check_times = append(load_check_times, elapsed.Seconds())
        log.Println("Num failures", failures)
        if err != nil {
            log.Fatal(err)
        }
        log.Println(result)

        var total_time []float64
        var sum_time float64
        N := 100000
        for i := 0; i < N; i++ {
            context["main.b"] = i
            context["main.a"] = rand.Intn(i + 1)
            context["main.c"] = rand.Intn(2 * i + 1)
            context["main.d"] = rand.Intn(i + 5)
            start = time.Now()
            _, _, err := checker.Check(context)
            elapsed = time.Since(start)
            //log.Printf("Checking properties took %s\n", elapsed)
            if err != nil {
                log.Fatal(err)
            }
            total_time = append(total_time, elapsed.Seconds())
            sum_time += elapsed.Seconds()
        }
        average := sum_time / float64(N)
        check_times = append(check_times, average)
        var sd float64
        for _, val := range total_time {
            sd += math.Pow(val - average, 2)
        }
        sd = math.Sqrt(sd/float64(N))
        check_time_sds = append(check_time_sds, sd)
    }

    f, err := os.Create("results.csv")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    _, err = f.WriteString("Filename,NumProps,BuildTime,LoadTime,CheckTime,SD\n")
    if err != nil {
        log.Fatal(err)
    }
    for i := 0; i < 4; i++ {
        _, err = f.WriteString(files[i] + "," + num_props[i] + fmt.Sprintf(",%f,%f,%f,%f\n", build_times[i], load_check_times[i], check_times[i], check_time_sds[i]))
        if err != nil {
            log.Fatal(err)
        }
    }
}
