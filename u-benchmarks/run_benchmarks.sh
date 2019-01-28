#!/bin/bash

go test -bench . > go1_10.txt
dgo test -bench . > dgo.txt

# TODO Run Record benchmark
# TODO Run Replay benchmark
