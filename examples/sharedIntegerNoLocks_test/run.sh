#!/bin/bash
dgo='/usr/local/go/bin/go'
PROCESSES=1
Program=sharedIntegerNoLock
SchedulerDir=github.com/DARA-Project/GoDist-Scheduler


killall nondet

case $1 in
"-k")
    exit
    ;;
"-t")
    difference=`diff $Program.record $Program.replay`
    echo $difference
    if [ "$difference" == "" ];then
        echo "PASS"
    else
        echo "FAIL"
        echo -e "DIFF ---->"
        echo $difference
    fi
    exit
    ;;
esac


#do i need to alloc the shared memory here?

echo INSTALLING THE SCHEDULER
$dgo install $SchedulerDir


dd if=/dev/zero of=./DaraSharedMem bs=400M count=1
chmod 777 DaraSharedMem
exec 666<> ./DaraSharedMem

echo "GoDist-Scheduler $1 $2 1> GlobalScheduler.out 2> GlobalScheduler.out &"
GoDist-Scheduler $1 $2 1> Global-Scheduler.out 2> Global-Scheduler.out &
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
    #sed -i '$ d' $Program.record
    ;;
"-r")
    ./$Program 1> $Program.replay 2> Local-Scheduler-$DARAPID.replay
    ;;
esac
