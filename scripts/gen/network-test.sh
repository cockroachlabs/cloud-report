#!/bin/bash

set -ex

f_force=''
f_wait=''
f_server=''
f_port=''
f_duration_latency=60
f_duration_throughput=720
f_server_mode=''
test_mode='cross-region'

machine_name="unknown machine"

function usage() {
  echo "$1
Usage: $0 [-f] [-w] [-s server] -p port
  -s server: connect to netserver running on specified server.
  -p port: port number for netserver process.
  -t <num>: throughput benchmark duration in seconds. (default: ${f_duration_throughput}s)
  -l <num>: latency benchmark duration in seconds. (default: ${f_duration_latency}s)
  -f: ignore existing pid file; override and rerun.
  -w: wait for currently running benchmark to complete.
  -d: duration to draw the throughput time series plot.
  -z: current machine type.
  -m: mode of network test. (default: cross-region)
  -S: start netserver.
"
  exit 1
}

while getopts 'fwks:p:t:l:d:m:z:S' flag; do
  case "${flag}" in
    s) f_server="${OPTARG}" ;;
    p) f_port="${OPTARG}" ;;
    t) f_duration_throughput="${OPTARG}" ;;
    l) f_duration_latency="${OPTARG}" ;;
    f) f_force='true' ;;
    w) f_wait='true' ;;
    m) test_mode="${OPTARG}" ;;
    z) machine_name="${OPTARG}" ;;
    S) f_server_mode='true' ;;
    *) usage "";;
  esac
done

logdir="$HOME/$test_mode-netperf-results"
report="${logdir}/$test_mode-netperf-results.log"
pidfile="$HOME/$test_mode-netperf-bench.pid"


if [ -n "$f_wait" ];
then
   # TODO(janexing): Show current Chicago time. Should be removed in the final version.
   TZ=UTC-6 date -R
   exec sh -c "
    ( test -f '$logdir/plot_success' ||
      (tail --pid \$(cat $pidfile) -f /dev/null && test -f '$logdir/plot_success')
    ) || (echo '$test_mode network benchmark did not complete successfully.  Check logs'; exit 1)"
fi


if [ -n "$f_server_mode" ];
then
  if [ -z "$f_port" ]
  then
    usage "-p argument required"
  fi
   exec sh -c "sudo lsof -i :$f_port >/dev/null || sudo netserver -p $f_port"
fi

if [ -f "$pidfile" ] && [ -z "$f_force" ]
then
  pid=$(cat "$pidfile")
  echo "Netperf benchmark already running (pid $pid)"
  exit 1
fi

trap "rm -f $pidfile" EXIT SIGINT
echo $$ > "$pidfile"

if [ -z "$f_server" ]
then
  usage "server and port args required. Use -s and -p argument to specify."
fi

rm -rf "$logdir"
mkdir "$logdir"

if [ -f $report ]
then
  rm $report
fi

if [ -z "$f_port" ]
then
  usage "-p argument required"
fi

# TODO: run clients on multiple nodes.
(
  echo "Using $(netperf -V)"
  # Latency
  sudo netperf -H "$f_server" -p "$f_port" -l "$f_duration_latency" -I 99,5  -t TCP_RR -- -O min_latency,mean_latency,P90_LATENCY,P99_LATENCY,max_latency,stddev_latency,transaction_rate
  # Throughput
  cd netperf/doc/examples && MACHINE_NAME=$machine_name TEST_MODE=$test_mode DRAW_PLOT=1 DURATION=$f_duration_throughput ./runemomniaggdemo.sh
  ) | tee "$report"

touch "$logdir/plot_success"
