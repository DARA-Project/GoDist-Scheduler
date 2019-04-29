#!/bin/bash

python plot_schedule.py time scalability/FastForwardTime/stats-1.csv scalability/FastForwardTime/stats-5.csv scalability/FastForwardTime/stats-10.csv scalability/FastForwardTime/stats-25.csv scalability/FastForwardTime/stats-50.csv scalability/FastForwardTime/stats-100.csv

python plot_schedule.py sched scalability/SlowSched/stats-1.csv scalability/SlowSched/stats-5.csv scalability/SlowSched/stats-10.csv scalability/SlowSched/stats-25.csv scalability/SlowSched/stats-50.csv scalability/SlowSched/stats-100.csv 
