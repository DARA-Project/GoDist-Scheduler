package main

import (
    "github.com/DARA-Project/GoDist-Scheduler/propchecker"
    "log"
    "time"
)

func main() {
    start := time.Now()
    checker, err := propchecker.NewChecker("./example/example.prop")
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
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Checking Properties took %s\n", elapsed)
	log.Println("All properties passed:",result)
	log.Println("Total property check failures:", len(*failures))

    start = time.Now()
	context["main.b"] = 15
    elapsed = time.Since(start)
	result, failures, err = checker.Check(context)
	if err != nil {
		log.Fatal(err)
	}
    log.Printf("Checking Properties took %s\n", elapsed)
	log.Println("All properties passed:",result)
	log.Println("Total property check failures:", len(*failures))
}
