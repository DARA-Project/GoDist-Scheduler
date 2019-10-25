package main

import (
    "github.com/DARA-Project/GoDist-Scheduler/propchecker"
    "os"
    "log"
)

func main() {
    checker, err := propchecker.NewChecker(os.Args[1])
    if err != nil {
        log.Fatal(err)
    }

    context := make(map[string]interface{})
    context["main.a"] = 15
    context["main.b"] = 20

    result, err := checker.Check(context)
    if err != nil {
        log.Fatal(err)
    }
    log.Println(result)

    context["main.b"] = 15
    result2, err := checker.Check(context)
    if err != nil {
        log.Fatal(err)
    }
    log.Println(result2)
}
