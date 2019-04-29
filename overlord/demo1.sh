#!/bin/bash

go run overlord.go -mode=record -optFile=configs/huawei-demo1.json
go run overlord.go -mode=replay -optFile=configs/huawei-demo1.json
