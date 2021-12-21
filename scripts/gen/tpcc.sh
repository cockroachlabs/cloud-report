#!/bin/bash

set -ex
pidfile="$HOME/tpcc-bench.pid"
f_force=''
f_wait=''
f_active=800
f_warehouses=1200
f_skip_load=''
f_duration="30m"
f_inc=200

function usage() {
  echo "$1
Usage: $0 [-f] [-w] [-s server] [pgurl,...]
  -f: ignore existing pid file; override and rerun.
  -w: wait for currently running benchmark to complete.
  -W: number of warehouses; default 2500
  -A: number of active warehouses; default 2500
  -I: warehouse increment; default 0 -- run tpcc once only once
  -s: skip loading stage
  -d: duration; default 30m
"
  exit 1
}

while getopts 'fwsW:A:I:d:' flag; do
  case "${flag}" in
    f) f_force='true' ;;
    w) f_wait='true' ;;
    W) f_warehouses="${OPTARG}" ;;
    A) f_active="${OPTARG}" ;;
    I) f_inc="${OPTARG}" ;;
    s) f_skip_load='true' ;;
    d) f_duration="${OPTARG}" ;;
    *) usage "";;
  esac
done

logdir="$HOME/tpcc-results"

if [ -n "$f_wait" ];
then
   exec sh -c "
    ( test -f '$logdir/success' ||
      (tail --pid \$(cat $pidfile) -f /dev/null && test -f '$logdir/success')
    ) || (echo 'TPC-C benchmark did not complete successfully.  Check logs'; exit 1)"
fi


if [ -f "$pidfile" ] && [ -z "$f_force" ];
then
  pid=$(cat $pidfile)
  echo "TPCC benchmark already running (pid $pid)"
  exit
fi

shift $(expr $OPTIND - 1 )
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

# Due to a bug in tpcc test (https://github.com/cockroachdb/cockroach/issues/73751),
# we have to set fixed upper boundary, if these limits are set above the actual
# limit a machine type can handle, the test will fail with COMMAND_ERROR, instead
# of exiting gracefully with error code or error message.
# These current set of limits are carefully tuned and pretty accurately reflect the
# limits of the machine types. You have to do the same tuning for new machine types
# until this bug is resolved with desired behavior.
vcpu=`grep -Pc '^processor\t' /proc/cpuinfo`
machinetype=`cat machinetype.txt`
if [ $vcpu -gt 16 ]
then
  f_active=2750
  f_inc=250
  if [[ $machinetype =~ r5.\.8xlarge || $machinetype =~ m5a\.8xlarge ]];
  then
    echo "Decreasing upper limit because of machine type $machinetype."
    f_warehouses=3250
  else
    f_warehouses=3500
  fi
elif [ $vcpu -eq 16 ]
then
  f_active=2500
  f_warehouses=3500
  f_inc=2500
else
  if [[ $machinetype =~ m6i.2xlarge ]];
  then
    echo "Decreasing upper limit because of machine type $machinetype."
    f_active=500
    f_warehouses=900
  fi
fi

if [ -z "$f_skip_load" ]
then
  echo "configuring the cluster for fast import..."
  ./cockroach sql --insecure --url "${pgurls[0]}" -e "
  SET CLUSTER SETTING kv.bulk_ingest.max_index_buffer_size = '2gib';
  SET CLUSTER SETTING kv.bulk_io_write.concurrent_addsstable_requests = 10;
  SET CLUSTER SETTING schemachanger.backfiller.max_buffer_size = '5GiB';
  SET CLUSTER SETTING kv.snapshot_recovery.max_rate = '128 MiB';
  SET CLUSTER SETTING kv.snapshot_rebalance.max_rate = '128 MiB';
  ";

  echo "importing..."
  ./cockroach workload fixtures import tpcc --warehouses="$f_warehouses" --active-warehouses="$f_active" "${pgurls[0]}"
  echo "done importing"
fi

if [[ $f_inc == 0 ]];
then
  f_inc=$f_warehouses
fi

for active in `seq $f_active $f_inc $f_warehouses`
do
  echo "Running TPCC: $active"
  report="${logdir}/tpcc-results-$active.txt"
./cockroach workload run tpcc \
    --warehouses="$f_warehouses"  \
    --active-warehouses="$active" \
    --ramp=1m --duration="$f_duration" \
    "${pgurls[@]}" > "$report"

    if [[ $(tail -1 "$report" | awk '{if($3 > 85 && $7 < 10000){print "pass"}}') != "pass" ]];
    then
      break
    fi
done

touch "$logdir/success"