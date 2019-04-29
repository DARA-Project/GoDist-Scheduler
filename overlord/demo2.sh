#!/bin/bash

go run overlord.go -mode=record -optFile=configs/huawei-demo2.json
go run overlord.go -mode=replay -optFile=configs/huawei-demo2.json
dgo run ../tools/shiviz_converter.go ../examples/MapCrash/Schedule.json ../tools/shiviz_logs/bug.log
cat ../tools/shiviz_logs/bug.log
