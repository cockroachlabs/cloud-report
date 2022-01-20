#!/bin/bash

set -ex
pidfile="$HOME/tpcc-bench.pid"
f_force=''
f_wait=''
f_active=4000
f_warehouses=6000
f_best_result=0
f_skip_load=''
f_duration="30m"
special_type=0
current_result_good=0

function usage() {
  echo "$1
Usage: $0 [-f] [-w] [-s server] [pgurl,...]
  -f: ignore existing pid file; override and rerun.
  -w: wait for currently running benchmark to complete.
  -W: number of warehouses; default 2500
  -A: number of active warehouses; default 2500
  -B: best known result, which will be used to kick off official run.
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
      f_best_result=5400
      f_active=5100
      f_warehouses=5700
      echo "Special type $1";;
    m5n\.2xlarge)
      f_best_result=1850
      f_active=1500
      f_warehouses=2200
      echo "Special type $1";;
    m5n\.8xlarge)
      f_best_result=4800
      f_active=4500
      f_warehouses=5100
      echo "Special type $1";;
    m5\.8xlarge)
      f_best_result=5400
      f_active=5100
      f_warehouses=5700
      echo "Special type $1";;
    Standard_D8s_v4)
      f_best_result=1800
      f_active=1500
      f_warehouses=2100
      echo "Special type $1";;
    c2-standard-8)
      f_best_result=800
      f_active=400
      f_warehouses=1000
      echo "Special type $1";;
  	*)
      special_type=0
      current_result_good=0
      f_best_result=0
      echo "Not a special type $1";;
  esac
}

function get_seconds() {
  end=`date +%s`
  runtime=$((end-$1))
  echo "Run time: $runtime seconds"
}

function load_data() {
  if [ -z "$f_skip_load" ]
  then
    begin_time=`date +%s`
    echo "configuring the cluster for fast import..."

#    This settings is documented in
#    https://www.cockroachlabs.com/docs/v21.2/performance-benchmarking-with-tpcc-medium#step-4-import-the-tpc-c-dataset
#    After experiment it would emit error, after checking with kv team some of
#    them are no longer valid options and this setting generally applies only
#    to very large data-set, which is not the case for cloud report run.
#
#    ./cockroach sql --insecure --url "${pgurls[0]}" -e "
#    SET CLUSTER SETTING rocksdb.ingest_backpressure.l0_file_count_threshold = 100;
#    SET CLUSTER SETTING rocksdb.ingest_backpressure.pending_compaction_threshold = '5 GiB';
#    SET CLUSTER SETTING schemachanger.backfiller.max_buffer_size = '5 GiB';
#    SET CLUSTER SETTING kv.snapshot_rebalance.max_rate = '128 MiB';
#    SET CLUSTER SETTING rocksdb.min_wal_sync_interval = '500us';
#    SET CLUSTER SETTING kv.range_merge.queue_enabled = false;
#    ";

    ./cockroach sql --insecure --url "${pgurls[0]}" -e "
    SET CLUSTER SETTING kv.bulk_ingest.max_index_buffer_size = '2gib';
    SET CLUSTER SETTING kv.bulk_io_write.concurrent_addsstable_requests = 10;
    SET CLUSTER SETTING schemachanger.backfiller.max_buffer_size = '5GiB';
    SET CLUSTER SETTING kv.snapshot_recovery.max_rate = '128 MiB';
    SET CLUSTER SETTING kv.snapshot_rebalance.max_rate = '128 MiB';
    ";

    echo "importing ..."
    # TODO, experiment importing with db backup and reload, to see which option
    # could generate faster loading speed.
    ./cockroach workload fixtures import tpcc --warehouses="$f_warehouses" --active-warehouses="$f_active" "${pgurls[0]}"
    echo "done importing"
    get_seconds $begin_time
  fi
}

# This function conducts initial exploration run when we don't know the initial
# performance range of a machine type, after the run we can obtain the starting
# point to start the next stage of formal runs.
function exploration_run() {
  echo "Start exploration run on $machinetype"

  is_special_type $machinetype
  if [[ $special_type -eq 0 ]];
  then
    if [ $vcpu -gt 16 ]
    then
        if [[ $machinetype =~ r5\.8xlarge || $machinetype =~ m5a\.8xlarge ]];
        then
          echo "Decreasing boundary range because of machine type $machinetype."
          f_active=2500
          f_warehouses=3500
        # Reg expression "r5.\.8xlarge" is for current r5 vcpu32 machine family
        # including r5a.8xlarge, r5b.8xlarge and r5n.8xlarge.
        elif [[ $machinetype =~ r5.\.8xlarge ]];
        then
          echo "Decreasing boundary range because of machine type $machinetype."
          f_active=2000
          f_warehouses=4000
        elif [[ $machinetype =~ c2-standard-30 ]];
        then
          echo "Decreasing boundary range because of machine type $machinetype."
          f_active=8000
          f_warehouses=10000
        elif [[ $machinetype =~ m6i\.8xlarge ]];
        then
          echo "Decreasing boundary range because of machine type $machinetype."
          f_active=3000
          f_warehouses=5000
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
      if [[ $machinetype =~ m6i\.2xlarge || $machinetype =~ m5n\.2xlarge ||$machinetype =~ n2d-highmem-8 ]];
      then
        echo "Decreasing upper limit because of machine type $machinetype."
        ((f_active-=200))
        ((f_warehouses-=800))
      elif [[ $machinetype =~ n2.*-highcpu-8 || $machinetype =~ c2-standard-8 ]];
      then
        echo "Decreasing upper limit because of machine type $machinetype."
        ((f_active-=500))
        ((f_warehouses-=1700))
      fi
    fi
  fi

  load_data

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
  f_inc=50
  has_good_result=0
  if [[ $special_type -eq 0 ]];
  then
    l=$f_active
    r=$f_warehouses
    begin_time=`date +%s`
    while [[ $l -le $r ]]
    do
        start=`date +%s`
        mid=$((l + ((r - l)/2)))
        report="${logdir}/tpcc-results-$mid.txt"
        ./cockroach workload run tpcc \
            --tolerate-errors \
            --warehouses="$mid"  \
            --active-warehouses="$mid" \
            --ramp=1m --duration="1m" \
            "${pgurls[@]}" > "$report"

        # awk uses lexicographic order by default when doing comparison, we
        # need to force it to use number comparison.
        prev_line=$(tail -2 "$report")
        if [[ "$prev_line" == *efc* ]] && [[ $(tail -1 "$report" | awk '{if(int($3) > 87){print "pass"}}') == "pass" ]];
        then
          echo "Test TPCC on $mid failed"
          ((r=mid-f_inc))
          current_result_good=0
        else
          echo "Test TPCC on $mid passed"
          ((l=mid+f_inc))
          current_result_good=1
          if [[ $mid -gt $f_best_result ]];
          then
            f_best_result=$mid
          fi
        fi
        get_seconds $start
    done

    echo "Exploration run time. "
    get_seconds $begin_time
  fi

  result_files=(`find "${logdir}"/ -maxdepth 1 -name "tpcc-results*.txt"`)
  if [ ${#result_files[@]} -gt 0 ];
  then
    rm "${logdir}"/tpcc-results*.txt
    echo "Deleted exploration run result."
  fi

  ((l=$f_best_result-$f_best_result%f_inc))
  if [ $current_result_good -eq 0 ]
  then
    ((l-=f_inc))
  fi

  # Short time exploration run target doesn't quite predict long time official
  # run target, need to make proper adjustment. This adjustment should make
  # most machine types' runs successful.
  ((f_best_result-=350))
}

while getopts 'fwsW:A:B:I:d:' flag; do
  case "${flag}" in
    f) f_force='true' ;;
    w) f_wait='true' ;;
    W) f_warehouses="${OPTARG}" ;;
    A) f_active="${OPTARG}" ;;
    B) f_best_result="${OPTARG}" ;;
    I) f_inc="${OPTARG}" ;;
    s) f_skip_load='true' ;;
    d) f_duration="${OPTARG}" ;;
    *) usage "";;
  esac
  echo "flag is $flag"
done

logdir="$HOME/tpcc-results"
echo "f_active:$f_active, f_warehouses:$f_warehouses, f_best_result:$f_best_result"
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
vcpu=$(grep -Pc '^processor\t' /proc/cpuinfo)
machinetype=$(cat machinetype.txt)

echo "f_active:$f_active, f_warehouses:$f_warehouses, f_best_result:$f_best_result"

if [ $f_best_result -eq 0 ]
then
  exploration_run
else
  load_data
fi

# If best result is defined or found in the previous test round, we start an
# official run directly.
#
# Because at present tpcc is pretty unstable where result can fluctuate
# within quite a range and varies differently between different runs, we retry
# a few times with a different boundary when the current run iteration failed,
# and failed the run if these retries failed to generate any valid result
# files.
#
# In this version, we try to use dynamic setting on step, with a large starting
# step, and change direction with step setting to half of last round's value
# with an opposite sign, for every iteration round test that failed, or passed
# with a negative step value. Hope this approach could make search converge more
# quickly for most cases, and a better success rate for first search round, to
# avoid expensive retries. Initial experiments provided a mixed result, with
# first round success rate improved, but mixed iteration numbers in the first
# round, we need more validations runs on larger set of machine types to
# validate if it is overall a better search solution.
#
# In order to achieve better comparison result across multiple machine
# types, we use a small increase of store size of 50.
#
if [ $f_best_result -gt 0 ]
then
  begin_time=`date +%s`
  f_inc=100
  l=$f_active
  for i in {1..4}
  do
    ((r=l+400))
    step=2*f_inc
    echo "Start official run round $i from $l-$r"
    current_result_good=1
    all_result_good=1
    has_result_good=0
    step=f_inc
    active=$l
    while [[ $active -le $r ]]
    do
      start=`date +%s`
      echo "Running TPCC on $active"
      report="${logdir}/tpcc-results-$active.txt"
      ./cockroach workload run tpcc \
        --tolerate-errors \
        --warehouses="$r"  \
        --active-warehouses="$active" \
        --ramp=3m --duration="30m" \
        "${pgurls[@]}" > "$report"

      get_seconds $start

      prev_line=$(tail -2 "$report")
      if [[ "$prev_line" != *efc* ]] || [[ $(tail -1 "$report" | awk '{if(int($3) > 87){print "pass"}}') != "pass" ]];
      then
        echo "TPCC on $active failed"
        current_result_good=0
        all_result_good=0
        ((step=-step/2))
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

        if [ step -lt 0 ]
        then
          ((step=-step/2))
        fi
      fi

      if [ ${step#-} -le 50 ]
      then
        break
      fi

      ((active+=step))
    done

    # Because the current tpcc harness is not stable, we need to make the
    # following adjustment to try to get the optimal result:
    # If all result are good, need to increase boundary and retry,
    # else if no good is good, need to decrease boundary and retry,
    # else already has optimal result, end the test.
    if [[ $all_result_good -eq 1 ]]
    then
      ((l+=2*f_inc))
      echo "All result are good, boundary may be lower than optimal, increase boundary to start with $l and retry."
    elif [[ $has_result_good -eq 0 ]]
    then
      ((l-=400))
      echo "No result is good, decrease boundary to start with $l and retry."
    else
      echo "Find optimal result, end the test."
      break
    fi
  done

  get_seconds $begin_time
fi

result_files=(`find "${logdir}"/ -maxdepth 1 -name "tpcc-results*.txt"`)
if [ -z "$result_files" ]
then
  echo "tpcc couldn't run, please adjust the boundary and run again."
elif [ $has_result_good -eq 0 ]
then
  echo "tpcc result has invalid efc value, please adjust the boundary and run again."
#elif [ $all_result_good -eq 1 ]
#then
#  echo "tpcc result has too high efc values, please adjust the boundary and run again."
fi