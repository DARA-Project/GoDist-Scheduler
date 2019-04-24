#!/bin/bash

ITERATION_LIST=( "1" "5" "10" "25" "50" "100" )
#ITERATION_LIST=( "1" )
MEM_FILE=../macro-benchmarks/scalability/FastForwardTime/memory.csv
EVENT_FILE=../macro-benchmarks/scalability/FastForwardTime/events.csv

echo "Memory\n" > $MEM_FILE
echo "Events,Sched_Events\n" > $EVENT_FILE
for iteration in "${ITERATION_LIST[@]}"
do
    export ITERATIONS=$iteration
    go run overlord.go -mode=bench -optFile=configs/SimpleFileReadIterative.json
    mv ../examples/SimpleFileReadIterative/stats.csv ../macro-benchmarks/scalability/FastForwardTime/stats-$ITERATIONS.csv
    stat --printf="%s" ../examples/SimpleFileReadIterative/Schedule.json >> $MEM_FILE
    echo "\n" >> $MEM_FILE
    dgo run ../tools/schedule_info.go ../examples/SimpleFileReadIterative/Schedule.json scale >> $EVENT_FILE
done
