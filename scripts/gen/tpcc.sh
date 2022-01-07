#!/bin/bash

set -ex
pidfile="$HOME/tpcc-bench.pid"
f_force=''
f_wait=''
f_active=1000
f_warehouses=2500
f_skip_load=''
f_duration="30m"
f_inc=50
special_type=0
current_result_good=0
best_result=0

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

function is_special_type() {
  special_type=1
  current_result_good=1
  case $1 in
  	c5n\.9xlarge)
      best_result=5400
      f_active=5100
      f_warehouses=5700
      echo "Special type $1";;
    m5n\.2xlarge)
      best_result=1850
      f_active=1500
      f_warehouses=2200
      echo "Special type $1";;
    m5n\.8xlarge)
      best_result=4800
      f_active=4500
      f_warehouses=5100
      echo "Special type $1";;
    m5\.8xlarge)
      best_result=5400
      f_active=5100
      f_warehouses=5700
      echo "Special type $1";;
    Standard_D8s_v4)
      best_result=1800
      f_active=1500
      f_warehouses=2100
      echo "Special type $1";;
  	*)
      special_type=0
      current_result_good=0
      best_result=0
      echo "Not a special type $1";;
  esac
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
  echo "Removing invalid result files..."
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
vcpu=$(grep -Pc '^processor\t' /proc/cpuinfo)
machinetype=$(cat machinetype.txt)

is_special_type $machinetype
if [[ $special_type -eq 0 ]];
then
  if [ $vcpu -gt 16 ]
  then
      # Reg expression "r5.\.8xlarge" is for current r5 vcpu32 machine family
      # including r5a.8xlarge, r5b.8xlarge and r5n.8xlarge.
      if [[ $machinetype =~ r5\.8xlarge || $machinetype =~ m5a\.8xlarge ]];
      then
        echo "Decreasing boundary range because of machine type $machinetype."
        f_active=2500
        f_warehouses=3500
      elif [[ $machinetype =~ r5.\.8xlarge || $machinetype =~ m6i\.8xlarge ]];
      then
        echo "Decreasing boundary range because of machine type $machinetype."
        f_active=2000
        f_warehouses=4000
      else
        f_active=3000
        f_warehouses=6500
      fi
  elif [ $vcpu -eq 16 ]
  then
    # We didn't run vcp16 types for 2022, this setting is just some educated
    # guess on possible initial boundary after the significantly modified
    # algorithm could explore the real boundary of machine types, need to further
    # tune and adjust to the boundary settings when conducting runs on vcpu16.
    f_active=2500
    f_warehouses=4500
  else
    # Besides the speical machine type rules we defined in `is_special_type`, we
    # still need this section to define another set of special machine type
    # rules with a narrower boundary, otherwise the tpcc run will fail due to
    # various error due to limited resources the machine types have.
    if [[ $machinetype =~ m6i\.2xlarge || $machinetype =~ m5n\.2xlarge || $machinetype =~ n2d-highmem-8 ]];
    then
      echo "Decreasing upper limit because of machine type $machinetype."
      ((f_active-=200))
      ((f_warehouses-=800))
    elif [[ $machinetype =~ n2.*-highcpu-8 ]];
    then
      echo "Decreasing upper limit because of machine type $machinetype."
      ((f_active-=500))
      ((f_warehouses-=1700))
    fi
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
  echo "importing ..."
  ./cockroach workload fixtures import tpcc --warehouses="$f_warehouses" --active-warehouses="$f_active" "${pgurls[0]}"
  echo "done importing"
fi

# Comment out since we don't have a scenario using this at present.
## if f_inc is not specified, then we set it equal to f_warehouses, this means
## we just need to run one iteration.
#if [[ $f_inc == 0 ]];
#then
#  f_inc=$f_warehouses
#fi

# We first run an exploration test round with binary search to find a right
# range of test boundary to use in the subsequent official run.
# For certain machine types the binary search result don't work well and will
# cause kv errors and end the test, so we need to handle them differently.
tpcc_files=""
has_good_result=0
if [[ $special_type -eq 0 ]];
then
  l=$f_active
  r=$f_warehouses
  while [[ $l -le $r ]]
  do
      mid=$((l + ((r - l)/2)))
      report="${logdir}/tpcc-results-$mid.txt"
      ./cockroach workload run tpcc \
          --warehouses="$mid"  \
          --active-warehouses="$mid" \
          --ramp=1m --duration="1m" \
          "${pgurls[@]}" > "$report"

      # awk uses lexicographic order by default when doing comparison, we
      # need to force it to use number comparison.
      if [[ $(tail -1 "$report" | awk '{if(int($3) > 87){print "pass"}}') != "pass" ]];
      then
        echo "Test TPCC on $mid failed"
        ((r=mid-f_inc))
        current_result_good=0
      else
        echo "Test TPCC on $mid passed"
        ((l=mid+f_inc))
        current_result_good=1
        if [[ $mid -gt best_result ]]
        then
          best_result=$mid
        fi
      fi
  done
fi

result_files=(`find "${logdir}"/ -maxdepth 1 -name "tpcc-results*.txt"`)
if [ ${#result_files[@]} -gt 0 ];
then
  rm "${logdir}"/tpcc-results*.txt
  echo "Deleted exploration run result."
fi


# if valid result is found in the previous test round, we start an official
# run after adjusting of the starting point to a multiple of one hundred.
# Because at present tpcc is pretty unstable where result can fluctuate
# within quite a range, we retry a few times with lower boundary for the
# official run stage before we call the test fail.
#
# In order to achieve better comparison result across multiple machine
# types, we use a small increase of store size of 50.
if [ $best_result -gt 0 ]
then
  ((l=$best_result-$best_result%f_inc))
  if [ $current_result_good -eq 0 ]
  then
    ((l-=f_inc))
  fi
  ((l-=f_inc*7))
  ((h=l+100))

  for i in {1..4}
  do
    echo "Start official run round $i from $l-$h"
    current_result_good=1
    all_result_good=1
    has_result_good=0
    for active in `seq $l $f_inc $h`
    do
      echo "Running TPCC on $active"
      report="${logdir}/tpcc-results-$active.txt"
      ./cockroach workload run tpcc \
        --warehouses="$h"  \
        --active-warehouses="$active" \
        --ramp=3m --duration="30m" \
        "${pgurls[@]}" > "$report"

        if [[ $(tail -1 "$report" | awk '{if(int($3) > 87){print "pass"}}') != "pass" ]];
        then
          echo "TPCC on $active failed"
          current_result_good=0
          all_result_good=0
          break
        else
          echo "TPCC on $active passed"
          current_result_good=1
          # There is a blocking tpcc issue that can end the test with
          # COMMAND_ERROR while a test round is running, this is caused by
          # various kv errors such as Error: error in newOrder error. To deal
          # with this issue and prevent loss of the existing success result, we
          # write the success file when we have a first success test round.
          if [[ $has_result_good -eq 0 ]]
          then
            touch "$logdir/success"
            has_result_good=1
          fi
        fi
    done

    # Because the current tpcc harness is not stable, we need to make the
    # following adjustment to try to get the optimal result:
    # If all result are good, need to increase boundary and retry,
    # else if no good is good, need to decrease boundary and retry,
    # else already has optimal result, end the test.
    if [[ $all_result_good -eq 1 ]]
    then
      ((l+=f_inc))
      ((h+=f_inc))
      echo "All result are good, boundary may be lower than optimal, increase boundary to $l-$h and retry."
    elif [[ $has_result_good -eq 0 ]]
    then
      ((h=l-f_inc))
      ((l=h-f_inc*5))
      echo "No result is good, decrease boundary to $l-$h and retry."
    else
      echo "Find optimal result, end the test."
      break
    fi
  done
fi

result_files=(`find "${logdir}"/ -maxdepth 1 -name "tpcc-results*.txt"`)
if [ -z "$result_files" ]
then
  echo "tpcc couldn't run, please adjust the boundary and run again."
elif [ $has_result_good -eq 0 ]
then
  echo "tpcc result has invalid efc value, please adjust the boundary and run again."
elif [ $all_result_good -eq 1 ]
then
  echo "tpcc result has too high efc values, please adjust the boundary and run again."
fi