#!/bin/bash

port=1337

SERVER=$1
if [ -z "$SERVER" ]
then
  echo "Usage: SERVER=<ip> $0"
  exit 1
fi


install_netperf() {
  host=$1
  cmd="sudo apt-get update && sudo apt-get install -y netperf"
  echo "Ensuring netperf up to date on $host"
  if [ "$host" = "localhost" ]
  then
    sh -c "$cmd" > /dev/null 2>&1
  else
    ssh "$host" "$cmd" > /dev/null 2>&1
  fi
}


install_netperf "$SERVER"
install_netperf "localhost"

echo "Using $(netperf -V)"
echo "Starting netserver on $SERVER:$port"
ssh "$SERVER" "sudo netserver -p $port"

echo "Starting netperf client: 10 iterations, 30 seconds each"
(
  for i in $(seq 1 10)
  do
    netperf -H "$SERVER" -p "$port" -t TCP_RR -l 30 -- -o min_latency,max_latency,mean_latency
  done
) | tee "netperf-results.log"


