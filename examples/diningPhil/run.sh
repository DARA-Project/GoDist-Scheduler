#!/bin/bash
# diningPhil/run.sh controls the exection of the dining philosophers
# example diningPhilosophers runs on an arbetrary number of hosts, the
# communication pattern follows Host_i-1 <--> Host_i <--> Host_i+1
# That is, every host has a neighbour, and only communicates with that
# neighbour

Hosts=3
BasePort=6000
DINV=$GOPATH/src/bitbucket.org/bestchai/dinv
DARA=$GOPATH/src/github.com/DARA-Project/GoDist-Scheduler
testDir=$DARA/examples/diningPhil
dgo=/usr/local/go/bin/go
P1=diningphilosopher.go
Original=original

LOGSERVER="localhost:17000"

function installDinv {
    echo "Install dinv"
    cd $DINV
    $dgo install
    cd $testDir
}

function runTestPrograms {
    cd $testDir
    pwd
    for (( i=0; i<Hosts; i++))
    do
        let "hostPort=i + BasePort"
        let "neighbourPort= (i+1)%Hosts + BasePort"

        export DINV_HOSTNAME="localhost:$hostPort"
        export DINV_LOG_STORE="localhost:17000"
        export DINV_PROJECT="phil"
        $dgo run diningphilosopher.go -mP $hostPort -nP $neighbourPort &
    done
    sleep 60
    kill `ps | pgrep dining | awk '{print $1}'`
}


installDinv
#instrument $P1
runTestPrograms
#runLogMerger
#time runDaikon
#if [ "$1" == "-d" ];
#then
#    exit
#fi
#cleanUp
