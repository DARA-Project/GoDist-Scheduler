#!/bin/bash

DARAPID=1 ./dphil -mP 9500 -nP 9501 &
DARAPID=2 ./dphil -mP 9501 -nP 9502 &
DARAPID=3 ./dphil -mP 9502 -nP 9500