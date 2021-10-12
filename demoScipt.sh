#!/bin/bash

RANDOM=$1
seudoRunningTIme=$((1 + $RANDOM % 10))

echo "Running program $seudoRunningTIme s"

sleep $seudoRunningTIme
