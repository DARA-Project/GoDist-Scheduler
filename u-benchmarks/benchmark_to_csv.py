#!/usr/bin/env python2

import sys

if len(sys.argv) != 3:
    print "Usage: %s [input_benchmark_file] [output_csv_file]" % (sys.argv[0])
    sys.exit(1)

entries = []

with open(sys.argv[1]) as bm_file:
    lines = bm_file.readlines()
    for line in lines:
        elem = line.split()
        if len(elem) != 4 or elem[3] != "ns/op":
            continue
        entries.append((elem[0][9:-2], elem[1], elem[2]))

with open(sys.argv[2], "w+") as csv_file:
    csv_file.write("name,n_iter,latency_ns\n")
    for entry in entries:
        csv_file.write("%s,%s,%s\n" % entry)
