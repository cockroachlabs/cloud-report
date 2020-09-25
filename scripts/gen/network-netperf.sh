#!/bin/bash

port=1337

SERVER=$1
if [ -z "$SERVER" ]
then
  echo "Usage: SERVER=<ip> $0"
  exit 1
fi

logdir="$HOME/netperf-results"
rm -rf "$logdir"
mkdir "$logdir"
report="${logdir}/netperf-results.log"
exec &> >(tee -a "$logdir/script.log")
set -ex

echo "Using $(netperf -V)"
echo "Starting netserver on $SERVER:$port"
ssh "$SERVER" "sudo netserver -p $port"

# TODO: run clients on multiple nodes.
(
  # Latency
  netperf "$SERVER" -p "$port" -l 60 -I 99,5  -t TCP_RR -- -O min_latency,mean_latency,P90_LATENCY,P99_LATENCY,max_latency,stddev_latency,transaction_rate
  # Throughput
  netperf "$SERVER" -p "$port" -l 60 -I 99,5 -t OMNI -- -O LSS_SIZE_END,RSR_SIZE_END,LSS_SIZE,ELAPSED,THROUGHPUT,THROUGHPUT_UNITS
) | tee "$report"
