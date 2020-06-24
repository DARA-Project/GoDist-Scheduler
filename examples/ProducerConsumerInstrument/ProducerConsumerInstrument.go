package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

type Consumer struct {
	msgs *chan int
}

// NewConsumer creates a Consumer
func NewConsumer(msgs *chan int) *Consumer {
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:19:21")
	return &Consumer{msgs: msgs}
}

// consume reads the msgs channel
func (c *Consumer) consume() {
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:25:27")
	fmt.Println("[consume]: Started")
	for {
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:28:103")
		msg := <-*c.msgs
		fmt.Println("[consume]: Received:", msg)
	}
}

// Producer definition
type Producer struct {
	msgs	*chan int
	done	*chan bool
}

// NewProducer creates a Producer
func NewProducer(msgs *chan int, done *chan bool) *Producer {
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:42:103")
	return &Producer{msgs: msgs, done: done}
}

// produce creates and sends the message through msgs channel
func (p *Producer) produce(max int) {
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:48:103")
	fmt.Println("[produce]: Started")
	for i := 0; i < max; i++ {
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:51:103")
		fmt.Println("[produce]: Sending ", i)
		*p.msgs <- i
	}
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:55:55")
	*p.done <- true
	fmt.Println("[produce]: Done")
}

func main() {
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:61:103")

	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

	max := flag.Int("n", 5, "defines the number of messages")

	flag.Parse()

	if *cpuprofile != "" {
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:77:103")
		f, err := os.Create(*cpuprofile)
		if err != nil {
			runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:80:84")
			log.Fatal("could not create CPU profile: ", err)
		}
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:83:86")
		if err := pprof.StartCPUProfile(f); err != nil {
			runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:85:86")
			log.Fatal("could not start CPU profile: ", err)
		}
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:88:84")
		defer pprof.StopCPUProfile()
	}
	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:91:105")

	var msgs = make(chan int)	// channel to send messages
	var done = make(chan bool)	// channel to control when production is done

	go NewProducer(&msgs, &done).produce(*max)

	go NewConsumer(&msgs).consume()

	<-done

	runtime.DaraLog("LeftElements", "main.leftElements", runtime.NumSendings(msgs) - runtime.NumDeliveries(msgs))

	runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:97:119")

	if *memprofile != "" {
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:100:102")
		f, err := os.Create(*memprofile)
		if err != nil {
			runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:103:112")
			log.Fatal("could not create memory profile: ", err)
		}
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:106:115")
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:109:119")
			log.Fatal("could not write memory profile: ", err)
		}
		runtime.ReportBlockCoverage("../examples/ProducerConsumerInstrument/ProducerConsumerInstrument.go:112:111")
		f.Close()
	}
}
