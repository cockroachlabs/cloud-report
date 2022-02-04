#!/bin/bash

set -ex
pidfile="$HOME/tpcc-bench.pid"
f_force=''
f_wait=''
f_active=0
f_warehouses=10000
f_active_per_core=125
f_skip_load=''
f_load_args=''
f_duration="30m"

function usage() {
  echo "$1
Usage: $0 [-f] [-w] [-s server] [pgurl,...]
  -f: ignore existing pid file; override and rerun.
  -w: wait for currently running benchmark to complete.
  -W: number of warehouses; default 2500
  -A: number of starting active warehouses
  -s: skip loading stage
  -L: extra args for load
  -a: number of active warehouses per core
  -d: duration; default 30m
"
  exit 1
}

while getopts 'fwsW:A:d:a:L:' flag; do
  case "${flag}" in
    f) f_force='true' ;;
    w) f_wait='true' ;;
    W) f_warehouses="${OPTARG}" ;;
    A) f_active="${OPTARG}" ;;
    a) f_active_per_core="${OPTARG}" ;;
    s) f_skip_load='true' ;;
    d) f_duration="${OPTARG}" ;;
    L) f_load_args="${OPTARG}" ;;
    *) usage "";;
  esac
done

logdir="$HOME/tpcc-results"

if [ -n "$f_wait" ];
then
  exec sh -c "
    ( test -f '$logdir/success' ||
      (test -f \"$pidfile\" && tail --pid \"$(cat $pidfile)\" -f /dev/null && test -f '$logdir/success')
    ) || (echo 'TPC-C benchmark did not complete successfully.  Check logs'; exit 1)"
fi

echo "f_load_args:[$f_load_args]"
echo "f_active_per_core:[$f_active_per_core]"


if [ -f "$pidfile" ] && [ -z "$f_force" ];
then
  pid=$(cat "$pidfile")
  echo "TPCC benchmark already running (pid $pid)"
  exit
fi

shift $((OPTIND - 1 ))
pgurls=("$@")

if [[ ${#pgurls[@]} == 0 ]];
then
  usage "list of pgurls required"
fi

trap "rm -f $pidfile" EXIT SIGINT
echo $$ > "$pidfile"

rm -rf "$logdir"
mkdir "$logdir"
exec &> >(tee -a "$logdir/script.log")

cd "$HOME"

if [ -z "$f_skip_load" ]
then
  #./cockroach sql --insecure --url "${pgurls[0]}" -e "
  # SET CLUSTER SETTING kv.snapshot_recovery.max_rate = '512 MiB';
  # SET CLUSTER SETTING kv.snapshot_rebalance.max_rate = '512 MiB';
	# SET CLUSTER SETTING admission.kv.enabled,=false;
	# SET CLUSTER SETTING admission.sql_kv_response.enabled=false;
	# SET CLUSTER SETTING admission.sql_sql_response.enabled=false;
  #";
  echo "Loading TPCC fixture for $f_warehouses warehouses ..."
  ./cockroach workload fixtures load tpcc --checks=false --warehouses="$f_warehouses" $f_load_args "${pgurls[0]}"
  echo "done loading"
fi

if (( f_active == 0 ))
then
  # Scale active warehouse count by f_active_per_core * number of CPUs.
  f_active=$(( f_active_per_core * $(cat /proc/cpuinfo | grep processor | wc -l) ))
  if (( f_active > f_warehouses ))
  then
    f_active=0
  fi
fi

report="${logdir}/tpcc-results-$f_active.txt"
./cockroach workload run tpcc \
  --warehouses="$f_warehouses" --active-warehouses="$f_active" --ramp=5m --duration="$f_duration" --tolerate-errors --wait=0 \
  "${pgurls[@]}" > "$report"
