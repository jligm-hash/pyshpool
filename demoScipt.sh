#!/bin/bash

RANDOM=$1
seudoRunningTime=$((1 + $RANDOM % 10))

echo "Running program $seudoRunningTime s"

sleep $seudoRunningTime
