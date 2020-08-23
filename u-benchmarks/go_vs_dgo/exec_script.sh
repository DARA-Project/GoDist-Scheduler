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

start_program() {
    if [ "$DARA_MODE" = "explore" ]
    then
        while [ ! -f ./explore_finish ]
        do
            launch_program
        done    
    else
        launch_program
    fi
}

if [ -z "$BENCH_RECORD" ]
then
	start_program
else
    date +"%s%6N" > record.tmp
    start_program
    date +"%s%6N" >> record.tmp
fi
# Bring back GoDist-Scheduler to foreground
fg
RC=$?
if [ $RC != 0 ]; then
    exit 0
fi
rm ./DaraSharedMem
