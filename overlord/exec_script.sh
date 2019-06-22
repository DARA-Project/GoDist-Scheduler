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

launch_program() {
    if [ -z "$RUN_SCRIPT" ]
    then
        ./$PROGRAM
    else
        ./$RUN_SCRIPT
    fi
}

if [ -z "$BENCH_RECORD" ]
then
	launch_program
else
    date +"%s%6N" > record.tmp
    launch_program
    date +"%s%6N" >> record.tmp
fi
# Bring back GoDist-Scheduler to foreground
fg
RC=$?
if [ $RC != 0 ]; then
    exit 0
fi
