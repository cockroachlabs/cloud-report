#!/bin/bash

sudo apt-get update
sudo apt-get install iperf -y
iperf --server --len=128k > network-iperf-server.log &
sleep 90
IPERFPID=$(pidof iperf)
kill -2 $IPERFPID