#!/bin/bash

#path to the insturmented go runtime
dgo='/usr/local/go/bin/go'
#The number of processes to run in the runtime
PROCESSES=1
#Name of test program
Program=sharedIntegerChannelRandom
#Scheduler Directory
SchedulerDir=github.com/DARA-Project/GoDist-Scheduler

#Kills all prior running instacnes of the program
killall $Program

#Run with -k to kill all running instances
if [ $1 == "-k" ];then
    exit
fi


#Install the scheduler
echo INSTALLING THE SCHEDULER
$dgo install $SchedulerDir

#Setup shared memory of size 400M
dd if=/dev/zero of=./DaraSharedMem bs=400M count=1
#make the memory accessabe
chmod 777 DaraSharedMem
#open a file discriptor  [666] to the shared memory
exec 666<> ./DaraSharedMem

#start the scheduler, write output to s.out, usefull to moniter with 
#tail -f s.out

#args $1 $2 order does not matter, used to specify number procs / read||write / manual
#read sched documentation fo command line info
echo "GoDist-Scheduler $1 $2 1>s.out 2>s.out &"
#Run scheduler in the background
GoDist-Scheduler $1 $2 1> s.out 2> s.out &
sleep 2

#wait 2 seconds to make sure the scheduler has locked all the data to
#stop the system from making grogress.
##TODO send a signal back to the script from the scheduler to make this progress

sleep 2

#compile the program with DGo
$dgo build $Program.go
##Turn on dara
##Sets the number of allowed goroutines to 1
export GOMAXPROCS=1
##Toggles the use of Dara, DARAON=False is the default scheduler
export DARAON=true
##The PID of the running program, not allowed to be duplicated on a single run
export DARAPID=1
#run the program
./$Program

