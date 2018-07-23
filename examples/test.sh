#!/bin/bash -x
dgo='/usr/local/go/bin/go'
SchedulerDir=github.com/DARA-Project/GoDist-Scheduler

testprogram=sharedIntegerNoLocks
silent=/dev/null


#COLORS
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASS=true


function RunTestCase {
    Program=$2

    killall $2 >$silent 2>&1
    killall GoDist-Scheduler >$silent 2>&1

    case $1 in
    "-k")
        return
        ;;
    "-c")
        rm *.replay         $2 > $silent 2>&1
        rm *.record         $2 > $silent 2>&1
        rm *.out            $2 > $silent 2>&1
        rm Schedule.json    $2 > $silent 2>&1
        rm DaraSharedMem    $2 > $silent 2>&1
        return
        ;;
    "-t")
        difference=`diff record.master replay.master`
        if [ "$difference" == "" ];then
            printf "${GREEN}PASS${NC}\n"
            PASS=true
            return
        else
            printf "${RED}FAIL${NC}\n"
            echo -e "DIFF ---->"
            echo $difference
            PASS=false
            return
        fi
        return
        ;;
    esac

    pwd
    dd if=/dev/zero of=./DaraSharedMem bs=1000M count=1
    chmod 777 DaraSharedMem
    exec 666<> ./DaraSharedMem


    export DARAON=false
    GoDist-Scheduler $1 -procs=$PROCESSES -rl=true -project=phil 1> ../Global-Scheduler.moniter 2> ../Global-Scheduler.moniter &
    schedpid=$!
    sleep 2


    #$1 is either -w (record) or -r (replay)
    #TODO restrict this to the config.bash?
    $dgo build $Program.go
    ##Turn on dara
    export GOMAXPROCS=1
    export DARATESTPEERS=$PROCESSES


    #Run the required test
    case $1 in
    "-w")
        #for i in $(seq 1 $PROCESSES)
        #do
        #    export DARAPID=$i
        #    #record an execution
        #    ./$Program 1> $Program-$DARAPID.record 2> Local-Scheduler-$DARAPID.record &
        #done
        RecordExecution
        wait $schedpid
        cat $Program-[0-9].record > record.master
        ;;


    "-r")
        #for i in $(seq 1 $PROCESSES)
        #do
        #    export DARAPID=$i
        #    #record an execution
        #    ./$Program 1> $Program-$DARAPID.replay 2> Local-Scheduler-$DARAPID.replay &
        #done
        ReplayExecution
        wait $schedpid
        cat $Program-[0-9].replay > replay.master
        ;;
    esac
}


#do i need to alloc the shared memory here?
echo INSTALLING THE SCHEDULER
$dgo install $SchedulerDir
echo BUILDING THE SCHEDULER
$dgo build $SchedulerDir
echo "---------------TESTS-------------------"


ulimit -n 70000
tests=""

if [ $1 == "-all" ]; then
    tests="*_test"
elif [ $1 != "" ]; then
    tests=$1
else
    echo "Argument to test should either be -all or the name of a directory"
fi

for testprogramdir in $tests; do
    #testprogramdir=broadcastUDPHashThread_test
    cd $testprogramdir

    source config.bash

    testprogram=`echo $testprogramdir | cut -d'_' -f 1`
    pwd
    printf "%-30s" "$testprogram"
    

    RunTestCase -w $testprogram
    RunTestCase -k $testprogram > $silent

    RunTestCase -r $testprogram
    RunTestCase -k $testprogram > $silent 2>&1

    RunTestCase -t $testprogram 
    if [ $PASS ]; then
        echo not cleaning
        #RunTestCase -c $testprogram
    else
        exit -1
    fi

    #break
    cd ..

done
