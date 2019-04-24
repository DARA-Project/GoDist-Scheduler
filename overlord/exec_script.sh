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

if [[ -z $BENCH_RECORD ]]; then
    date +"%s%6N" > record.tmp
    ./$PROGRAM
    date +"%s%6N" >> record.tmp
else
	./$PROGRAM
fi
# Bring back GoDist-Scheduler to foreground
fg
RC=$?
if [ $RC != 0 ]; then
    exit 0
fi
