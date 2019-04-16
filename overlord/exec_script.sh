#!/bin/bash

# Enable Job Control
set -m
exec 666<> ./DaraSharedMem
GoDist-Scheduler $1 &
sleep 2

export GOMAXPROCS=1
export DARAON=true
export DARA_PROFILING=true
export DARAPID=1

./$PROGRAM
# Bring back GoDist-Scheduler to foreground
fg
