package main

import (
    "github.com/DARA-Project/GoDist-Scheduler/propchecker"
    "os"
    "log"
    "time"
)

func main() {
    start := time.Now()
    checker, err := propchecker.NewChecker(os.Args[1])
    elapsed := time.Since(start)
    log.Printf("Parsing and Building took %s", elapsed)
    if err != nil {
        log.Fatal(err)
    }

    context := make(map[string]interface{})
    context["main.a"] = 15
    context["main.b"] = 20

    start = time.Now()
    result, failures, err := checker.Check(context)
    elapsed = time.Since(start)
    log.Printf("Checking Properties took %s\n", elapsed)
    log.Println("Num failures", failures)
    if err != nil {
        log.Fatal(err)
    }
    log.Println(result)

    context["main.b"] = 15
    for i := 0; i < 100; i++ {
        context["main.b"] = i
        start = time.Now()
        _, _, err := checker.Check(context)
        elapsed = time.Since(start)
        log.Printf("Checking properties took %s\n", elapsed)
        if err != nil {
            log.Fatal(err)
        }
    }
}
