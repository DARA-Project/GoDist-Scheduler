# GoDist-Scheduler
![scheduler](https://i.stack.imgur.com/bzDfr.png)

GoDist-Scheduler is a global scheduler for model checking distributed systems. The GoDist-Scheduler communicates to 
insturmented version of the Golang runtime via shared memory. The insturmented runtime can be found 
[here](https://github.com/DARA-Project/GoDist).

## Getting Started

To install GoDist-Scheduler you must have a working installation of [Go](https://golang.org/doc/install).
Second [GoDist](https://github.com/DARA-Project/GoDist) (insturmented go runtime) must be installed to `/usr/local/go`
in future releases this location with be configurable.
Once you have that run `go install` in the main directory. For a first experience with running the model checker
explore any of the examples in the *example* directory. The script run.sh in each example with run the model checker.
