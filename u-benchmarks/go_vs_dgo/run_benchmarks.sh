#!/bin/bash

go test -bench go/ > go1_10.txt
dgo test -bench dgo > dgo.txt

# TODO Run Record benchmark
# TODO Run Replay benchmark
