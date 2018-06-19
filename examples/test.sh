#!/bin/bash
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

    killall $2 > $silent 2>&1
    killall GoDist-Scheduler > $silent 2>&1

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
        difference=`diff $Program.record $Program.replay`
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


    dd if=/dev/zero of=./DaraSharedMem bs=400M count=1 > $silent 2>&1
    chmod 777 DaraSharedMem
    exec 666<> ./DaraSharedMem


    export DARAON=false
    GoDist-Scheduler $1 1> ../Global-Scheduler.moniter 2> ../Global-Scheduler.moniter &
    sleep 2


    #$1 is either -w (record) or -r (replay)
    $dgo build $Program.go
    ##Turn on dara
    export GOMAXPROCS=1
    export DARAON=true
    export DARAPID=1


    #Run the required test
    case $1 in
    "-w")
        #record an execution
        ./$Program 1> $Program.record 2> Local-Scheduler-$DARAPID.record
        sed -i '$ d' $Program.record
        ;;
    "-r")
        ./$Program 1> $Program.replay 2> Local-Scheduler-$DARAPID.replay
        ;;
    esac
}


#do i need to alloc the shared memory here?
echo INSTALLING THE SCHEDULER
$dgo install $SchedulerDir
echo BUILDING THE SCHEDULER
$dgo build $SchedulerDir

for testprogramdir  in *_test; do
    cd $testprogramdir

    testprogram=`echo $testprogramdir | cut -d'_' -f 1`
    printf "%-30s" "$testprogram"
    

    RunTestCase -w $testprogram
    RunTestCase -k $testprogram
    
    sleep 1

    RunTestCase -r $testprogram
    RunTestCase -k $testprogram

    RunTestCase -t $testprogram
    if [ $PASS ]; then
        RunTestCase -c $testprogram
    else
        exit -1
    fi

    #TODO START HERE TOMORROW, you are about to standardize the output
    #of all the test cases so that when they pass or fail the output of
    #the test is either cleaned or retured to the developer who ran the
    #test. After that each of the run scripts in the example programs
    #can be removed. The next part is to extend this script to handel
    #multiple processes and then write tests for that.
    cd ..

done
