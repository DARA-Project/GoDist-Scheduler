#!/bin/bash

exec 666<> ./DaraSharedMem
GoDist-Scheduler $1 &
sleep 2

export GOMAXPROCS=1
export DARAON=true
export DARA_PROFILING=true
export DARAPID=1

./$PROGRAM
