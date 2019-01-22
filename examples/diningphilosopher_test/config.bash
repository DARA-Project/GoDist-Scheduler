#!/bin/bash -x
# diningPhil/run.sh controls the exection of the dining philosophers
# example diningPhilosophers runs on an arbetrary number of hosts, the
# communication pattern follows Host_i-1 <--> Host_i <--> Host_i+1
# That is, every host has a neighbour, and only communicates with that
# neighbour

export PROCESSES=3
export DINV_LOG_STORE="localhost:17000"
export DINV_PROJECT="phil"
export BasePort=6000
DINV=$GOPATH/src/bitbucket.org/bestchai/dinv
DARA=$GOPATH/src/github.com/DARA-Project/GoDist-Scheduler
testDir=$DARA/examples/diningphilosopher_test
dgo=/usr/bin/dgo
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
    for (( i=0; i<PROCESSES; i++))
    do
        let "hostPort=i + BasePort"
        let "neighbourPort= (i+1)%PROCESSES + BasePort"

        export DINV_HOSTNAME="localhost:$hostPort"
        $dgo run diningphilosopher.go -mP $hostPort -nP $neighbourPort &
    done
    sleep 15
    kill `ps | pgrep dining | awk '{print $1}'`
}

function RecordExecution {
    cd $testDir
    pwd
    $dgo build diningphilosopher.go
    for i in $(seq 1 $PROCESSES)
    do
        echo "$i $i $i $i"
        let "k=i - 1"
        let "hostPort=k + BasePort"
        let "neighbourPort= (k+1)%PROCESSES + BasePort"

        export DINV_HOSTNAME="localhost:$hostPort"
        export DARAPID=$i
        export DARAON=true
        #record an execution
        ./diningphilosopher -mP $hostPort -nP $neighbourPort 1> diningphilosoper-$DARAPID.record 2> Local-Scheduler-$DARAPID.record &
        export DARAON=false
    done
}

function ReplayExecution {
    cd $testDir
    pwd
    $dgo build diningphilosopher.go
    for i in $(seq 1 $PROCESSES)
    do
        echo "$i $i $i $i"
        let "k=i - 1"
        let "hostPort=k + BasePort"
        let "neighbourPort= (k+1)%PROCESSES + BasePort"

        export DINV_HOSTNAME="localhost:$hostPort"
        export DARAPID=$i
        export DARAON=true
        #record an execution
        ./diningphilosopher -mP $hostPort -nP $neighbourPort 1> diningphilosoper-$DARAPID.replay 2> Local-Scheduler-$DARAPID.replay &
        export DARAON=false
    done
}
