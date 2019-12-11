#!/bin/bash

sudo apt-get install iperf -y
iperf --server --len=128k | tee network-iperf-server.log &
sleep 100
IPERFPID=$(pidof iperf)
if ! [ -z "$IPERFPID" ]
then
      kill -2 $IPERFPID
fi
