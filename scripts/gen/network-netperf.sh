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
echo "Starting netperf client: 10 iterations, 30 seconds each"
(
  for i in $(seq 1 10)
  do
    netperf -H "$SERVER" -p "$port" -t TCP_RR -l 30 -- -o min_latency,max_latency,mean_latency
  done
) | tee "$report"
