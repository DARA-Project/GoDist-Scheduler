#!/bin/bash

# For converting properties into executable functions
go get github.com/novalagung/go-eek
# Dinv for now.....But it shall be gone as soon as instrumentation is updated
go get bitbucket.org/bestchai/dinv/capture
# Delve for finding the full variables names in a given binary
go get github.com/go-delve/delve/pkg/proc
