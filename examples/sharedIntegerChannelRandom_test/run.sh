#!/bin/bash
dgo='/usr/bin/dgo'
PROCESSES=1
Program=sharedIntegerChannelRandom
SchedulerDir=github.com/DARA-Project/GoDist-Scheduler
export PATH="$PATH:$GOPATH/bin"

killall GoDist-Scheduler

if [ $1 == "-k" ];then
    exit
fi


#do i need to alloc the shared memory here?

echo INSTALLING THE SCHEDULER
$dgo install $SchedulerDir

rm DaraSharedMem
dd if=/dev/zero of=./DaraSharedMem bs=400M count=1
chmod 777 DaraSharedMem
exec 666<> ./DaraSharedMem

echo "GoDist-Scheduler $1 $2 1>s.out 2>s.out &"
GoDist-Scheduler $1 $2 1> s.out 2> s.out &
sleep 2


#$1 is either -w (record) or -r (replay)
$dgo build $Program.go

##Turn on dara
export GOMAXPROCS=1
export DARAON=true
export DARAPID=1
export DARA_PROFILING=true
./$Program
