#!/bin/bash
set -x
NAME_EXTRA=${NAME_EXTRA:=ori}

CLOUD="gce"
CLUSTER="$CRL_USERNAME-cyan-22-1"
TMUX_SESSION="workloads"
WEST_CLUSTER="${CLUSTER}-west"
west_cluster_created=''

# If env var NODES is not specified, set NODES to 4.
NODES=${NODES:=16}

TPCC_WAREHOURSE_PER_VCPU=${TPCC_WAREHOURSE_PER_VCPU:=125}

# We start different ports for testserver for the cross-region and intra-az network test.
CROSS_REGION_PORT=12865
INTER_AZ_PORT=1337

set -ex
scriptName=$(basename ${0%.*})
logdir="$(dirname $0)/../logs/${scriptName}"
mkdir -p "$logdir"

# Redirect stdout and stderr into script log file
exec &> >(tee -a "$logdir/driver-$NAME_EXTRA.log")

# Create roachprod cluster
function create_cluster() {
  roachprod create "$CLUSTER" -n $NODES --lifetime "720h" --clouds "$CLOUD" \
    --$CLOUD-machine-type "n2-standard-8" --gce-zones="us-central1-a,us-central1-b,us-central1-c"  --gce-pd-volume-type="pd-ssd" --gce-min-cpu-platform="Intel Cascade Lake" --local-ssd="false"   --gce-pd-volume-size="1000"   --gce-image="ubuntu-2004-focal-v20210927" \
    --label usage=long-running-cluster-22-1

  roachprod run "$CLUSTER" -- tmux new -s "$TMUX_SESSION" -d
  roachprod run "$CLUSTER" -- tmux set-option remain-on-exit on
}

# Create roachprod in us-west2
function create_west_cluster() {
  roachprod create "$WEST_CLUSTER" -u $USER -n 1 --lifetime "6h" --clouds "$CLOUD" \
    --$CLOUD-machine-type "n2-standard-8" --gce-zones="us-west1-a"  --gce-pd-volume-type="pd-ssd" --gce-min-cpu-platform="Intel Cascade Lake" --local-ssd="false"   --gce-pd-volume-size="1000"   --gce-image="ubuntu-2004-focal-v20210927" \
    --label usage=cloud-report-2022

  roachprod run "$WEST_CLUSTER" -- tmux new -s "$TMUX_SESSION" -d
  west_cluster_created=true
}

# Upload scripts to roachprod cluster
function upload_scripts() {
  roachprod run "$1" rm  -- -rf ./scripts
  roachprod put "$1" ./scripts scripts
  echo "n2-standard-8" > "machinetype.txt"
  roachprod put "$1" "machinetype.txt" "machinetype.txt"
  roachprod run "$1" chmod -- -R +x ./scripts
  roachprod put "$1" ./netperf ./netperf
  roachprod run "$1" chmod -- -R +x ./netperf
}

# Load the cockroach binary to roachprod cluster
function load_cockroach() {
  roachprod run "$1" "rm -f ./cockroach"
  if [ -z "$cockroach_binary" ]
  then
    cockroach_version=$(curl -s -i https://edge-binaries.cockroachdb.com/cockroach/cockroach.linux-gnu-amd64.LATEST |grep location|awk -F"/" '{print $NF}')
    echo "WARN: staging a stable cockroach binary from master with hash: 5ac733bb4927020bc1c52da24b2591742fde8e1f"
    roachprod stage "$1" cockroach
  elif [[ $cockroach_binary =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "INFO: staging release version $cockroach_binary of cockroach binary"
    roachprod stage "$1" release "$cockroach_binary"
  else
    echo "WARN: staging unknown version of cockroach binary from local path: $cockroach_binary"
    roachprod put "$1" "$cockroach_binary" "cockroach"
  fi
}

# Start cockroach cluster on nodes [1-NODES-1].
function start_cockroach() {
  # Build --store flags based on the number of disks.
  # Roachprod adds /mnt/data1/cockroach by itself, so, we'll pick up the other disks
  for s in $(roachprod run "$CLUSTER":1 'ls -1d /mnt/data[2-9]* 2>/dev/null || echo')
  do
   stores="$stores --store $s/cockroach"
  done

  if [[ -z $stores ]]; then
    stores="--store=/mnt/data1/cockroach"
  fi

  if [[ $NODES == 2 ]]; then
  	roachprod start "$CLUSTER":1 --args="$stores --cache=0.25 --max-sql-memory=0.4"
  else
  	roachprod start "$CLUSTER":1-$((NODES-1)) --args="$stores --cache=0.25 --max-sql-memory=0.4"
  fi
}

# Execute setup.sh script on the cluster to configure it
function setup_cluster() {
	roachprod run "$1" sudo ./scripts/gen/setup.sh "$CLOUD"
	roachprod run "$1":1 -- cpufetch -s legacy|awk -F"@" '{print $NF}'|tr -d ' '|awk NF > "$logdir"/"$1"_cpu_info.txt
  roachprod run "$1":1 -- lscpu  >> "$logdir"/"$1"_cpu_info.txt
}

# executes command on a host using roachprod, under tmux session.
function run_under_tmux() {
  local name=$1
  local host=$2
  local cmd=$3
  roachprod run $host -- tmux neww -t "$TMUX_SESSION" -n "$name" -d -- "$cmd"
}

# Benchmark scripts should execute a single benchmark
# and download results to the $logdir directory.
# results_dir returns date suffixed directory under logdir.
#
function results_dir() {
  echo "$logdir/$1.$(date +%Y%m%d.%T)-$NAME_EXTRA"
}

function copy_result_with_retry() {
  local node=$1
  local fetch_dir=$2
  local collect_cpu_info=$3

  # There is a random roachprod issue that we recently identified:
  # After a test finished successfully in a host, the following "roachprod get"
  # command didn't copy the result files correctly from the host node to the
  # client, so the target result directory ended up with all empty files.
  # This function will copy result files from host to client and check that
  # there is no empty result file in the target directory. It will retry the
  # copy step if the previous copy failed with any empty result files, the test
  # will fail if after retry it still couldn't copy file correctly.
  #
  target_dir=$(results_dir "$fetch_dir")
  for i in {1..3}
  do
    roachprod get "$node" "./$fetch_dir" "$target_dir"

    result_files=$(find "$target_dir" -empty -type f -name "*.log")
    if [ -z "$result_files" ]
    then
      echo "Test passed!"
      break
    fi
    echo "Copy file round "$i" failed, found empty result file(s):\n$result"
    sleep 5s
  done

  if [ ! -z "$result_files" ]
  then
    echo "Copy failed with empty result file(s) in "$target_dir", test failed!"
  fi

  if [ -n "$collect_cpu_info" ]
  then
     roachprod run "$CLUSTER" -- cpufetch -s legacy > "$target_dir/cpu_info.txt"
     roachprod run "$CLUSTER" -- lscpu  >> "$target_dir/cpu_info.txt"
  fi
}

# Run CPU benchmark
function bench_cpu() {
  run_under_tmux "cpu" "$CLUSTER:1"  "./scripts/gen/cpu.sh $cpu_extra_args"
}

# Wait for CPU benchmark to finish and retrieve results.
function fetch_bench_cpu_results() {
  node="$CLUSTER":1
  roachprod run $node ./scripts/gen/cpu.sh -- -w
  copy_result_with_retry $node "coremark-results"
}

# Run FIO benchmark
function bench_io() {
  run_under_tmux "io" "$CLUSTER:1" "./scripts/gen/fio.sh $io_extra_args"
}

# Wait for FIO benchmark top finish and retrieve results.
function fetch_bench_io_results() {
  node="$CLUSTER":1
  roachprod run $node ./scripts/gen/fio.sh -- -w
  copy_result_with_retry $node "fio-results"
}

# Wait for Netperf benchmark to complete and fetch results.
function fetch_bench_net_results() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  target_dir=$(results_dir "netperf-results")
  if [ $NODES -eq 2 ]
  then
    node="$CLUSTER":1
  else
    node="$CLUSTER":3
  fi
  roachprod run $node ./scripts/gen/network-netperf.sh -- -w
  copy_result_with_retry $node "netperf-results"
}

# Run TPCC Benchmark
function bench_tpcc() {
  if [ $NODES -lt 2 ]; then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  #start_cockroach
  if [ $NODES -eq 2 ]; then
    pgurls=$(roachprod pgurl "$CLUSTER":1)
    run_under_tmux "tpcc" "$CLUSTER:2" "./scripts/gen/tpcc.sh $tpcc_extra_args $TPCC_EXTRA_ARGS ${pgurls[@]}"
  else
    pgurls=$(roachprod pgurl --secure "$CLUSTER":1-3,6-8,11-13)
    run_under_tmux "tpcc" "$CLUSTER:$NODES" "./scripts/gen/tpcc.sh $tpcc_extra_args $TPCC_EXTRA_ARGS ${pgurls[@]}"
  fi
}

function fetch_bench_tpcc_results() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  node="$CLUSTER":$NODES
  
  # Don't exist if the following section gives error.
  set +e
  roachprod run $node ./scripts/gen/tpcc.sh -- -w
  copy_result_with_retry $node "tpcc-results" "with_cpu_inf"
  set -e 
}

# modify_remote_hosts_on_client_node is to get the ip from the remote node, 
# write it into a local file, and mount it to the netperf/doc/examples folder 
# in the client node.
function modify_remote_hosts_on_client_node() {
  # client_node is the one to run TCP_RR and TCP_STREAM.
  local client_node=$1
  local server_node=$2
  # test_mode should be either cross-region or intra-az.
  local test_mode=$3

  local server_ip=$(roachprod ip "$server_node")
  if [ -z $server_ip ]
  then
    echo "cannot get server_ip FOR server (remote) node ($server_node) in network test"
    exit 1
  fi
  
  # Since linux doesn't allow ":" in filename, we replace the ":" in 
  # $server_node to "-".
  formatted_server_node=$(echo "${server_node//:/-}")
  echo "formatted_server_node=$formatted_server_node"
  
  # Save the ip address of the server node into the 
  # netperf/doc/examples/remote_hosts in the client node.
  local remote_host_file="${logdir}/${formatted_server_node}_${test_mode}_remote_hosts"
  printf "REMOTE_HOSTS[0]=$server_ip\nREMOTE_HOSTS[1]=$server_ip\nNUM_REMOTE_HOSTS=2\n" >"$remote_host_file"
  chmod 777 "$remote_host_file"
  roachprod run "$client_node" -- sudo chmod 777 -R netperf
  roachprod put "$client_node" "$remote_host_file" netperf/doc/examples/${test_mode}_remote_hosts
}

# get_best_number_streams is to run a netperf TCP_STREAM test with 
# gradually incrementing the number of streams until the aggregate throughput 
# converges (the latest 3 agg throughput's std < 0.3). The best number of 
# streams will be saved in a file "num_streams" in the client node.
function get_best_number_streams() {
  local client_node=$1
  local test_mode=$2
  echo "running getting best num of stream for $client_node"

  roachprod run "$client_node" -- "cd netperf/doc/examples && SEARCH_BEST_NUM_STREAMS=1 TEST_MODE=$test_mode ./runemomniaggdemo.sh"
  echo "get best number of stream for $client_node"
}

# run_netperf_between_server_client is to get the best number of streams 
# to run the throughput test between the server and client node, and run the
# netperf latency and throughput test. We start the netserver on the server node
# and run netperf command on the client node.
# Note that in the cross-region case, we set the east node as the client node, 
# and the west node as the server node. 
function run_netperf_between_server_client() {
  
  local client_node=$1
  local server_node=$2

  local PORT=$3
  local test_mode=$4
  local netperf_extra_args=$5

  local server_ip=$(roachprod ip $server_node)

  roachprod run $client_node sudo ./scripts/gen/network-setup.sh
  roachprod run $server_node sudo ./scripts/gen/network-setup.sh

  # Start netserver on the server node.
  # It may give error, but it only means that the netserver is already running
  # on the given port, so we should proceed when that happens.
  set +e
  roachprod run $server_node ./scripts/gen/network-test.sh -- -S -p $PORT -m $server_node
  set -e
  
  # Mount a file containing server's ip to the client node.
  modify_remote_hosts_on_client_node $client_node $server_node $test_mode
  get_best_number_streams $client_node $test_mode
  run_under_tmux "${test_mode}-net" $client_node "./scripts/gen/network-test.sh -s $server_ip -p $PORT -m $test_mode -z $CLOUD-n2-standard-8 $netperf_extra_args"
}

# Run intra-az Netperf benchmark. The test will be run the 1st and the 2nd
# nodes of the same cluster.
function bench_intra_az_net() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  local server_node="$CLUSTER":2
  local client_node="$CLUSTER":1

  run_netperf_between_server_client $client_node $server_node $INTER_AZ_PORT intra-az "$net_extra_args"
}

# Wait for Netperf benchmark to complete and fetch results.
function fetch_bench_intra_az_net_results() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  roachprod run ${CLUSTER}:1 ./scripts/gen/network-test.sh -- -w -m intra-az
  roachprod get ${CLUSTER}:1 ./intra-az-netperf-results $(results_dir "intra-az-netperf-results")
}

# bench_cross_region_net is run the cross-region network tests.
function bench_cross_region_net() {
  create_west_cluster
  upload_scripts "$WEST_CLUSTER"
  setup_cluster "$WEST_CLUSTER"

  run_netperf_between_server_client ${CLUSTER}:1 ${WEST_CLUSTER}:1 $CROSS_REGION_PORT cross-region $cross_region_net_extra_args
}

# fetch_bench_cross_region_net_results is to wait the cross-region network test
# to finish and the fetch the results from the server node.
function fetch_bench_cross_region_net_results() {
  roachprod run ${CLUSTER}:1 ./scripts/gen/network-test.sh -- -w -m cross-region
  roachprod get ${CLUSTER}:1 ./cross-region-netperf-results $(results_dir "cross-region-netperf-results")
}

# Destroy roachprod cluster
function destroy_cluster() {
  roachprod destroy "$CLUSTER"
  if [[ -n $west_cluster_created ]]; then
    roachprod destroy "$WEST_CLUSTER"
  fi
}

function usage() {
echo "$1
Usage: $0 [-b <bootstrap>]... [-w <workload>]... [-d] [-c cockroach_binary]
   -b: One or more bootstrap steps.
         -b create: creates cluster
         -b upload: uploads required scripts
         -b setup: execute setup script on the cluster
         -b all: all of the above steps
   -w: Specify workloads (benchmarks) to execute.
       -w cpu : Benchmark CPU
       -w io  : Benchmark IO
       -w ia_net : Benchmark Net. Please don't run "ia_net" and "cr_net" on the same cluster.
       -w cr_net : Benchmark Cross-region Net. Please don't run "ia_net" and "cr_net" on the same cluster.
       -w tpcc: Benchmark TPCC
       -w all : All of the above
   -c: Override cockroach binary to stage (local path to binary or release version)
   -r: Do not start benchmarks specified by -w.  Instead, resume waiting for their completion.
   -I: additional IO benchmark arguments
   -N: additional network benchmark arguments
   -C: additional CPU benchmark arguments
   -T: additional TPCC benchmark arguments
   -R: additional cross-region network benchmark arguments
   -n: override number of nodes in a cluster
   -d: Destroy cluster
"
exit 1
}

benchmarks=()
f_resume=''
do_create=''
do_upload=''
do_setup=''
do_destroy=''
io_extra_args=''
cpu_extra_args=''
tpcc_extra_args=' -L "--provider-override=gs --bucket-override=long-running-cluster-tpcc" '
intra_az_net_extra_args=''
cross_region_net_extra_args=''
cockroach_binary=''

while getopts 'c:b:w:dn:I:N:C:T:R:r' flag; do
  case "${flag}" in
    b) case "${OPTARG}" in
        all)
          do_create='true'
          do_upload='true'
          do_setup='true'
          do_cockroach='true'
        ;;
        create)    do_create='true' ;;
        upload)    do_upload='true' ;;
        setup)     do_setup='true' ;;
        *) usage "Invalid -b value '${OPTARG}'" ;;
       esac
    ;;
    c) cockroach_binary="${OPTARG}" ;;
    w) case "${OPTARG}" in
         cpu) benchmarks+=("bench_cpu") ;;
         io) benchmarks+=("bench_io") ;;
         ia_net) benchmarks+=("bench_intra_az_net") ;;
         cr_net) benchmarks+=("bench_cross_region_net") ;;
         tpcc) benchmarks+=("bench_tpcc") ;;
         all) benchmarks+=("bench_cpu" "bench_io" "bench_tpcc" "bench_cross_region_net") ;;
         *) usage "Invalid -w value '${OPTARG}'";;
       esac
    ;;
    d) do_destroy='true' ;;
    r) f_resume='true' ;;
    n) NODES="${OPTARG}" ;;
    I) io_extra_args="${OPTARG}" ;;
    C) cpu_extra_args="${OPTARG}" ;;
    T) tpcc_extra_args="${OPTARG}" ;;
    N) intra_az_net_extra_args="${OPTARG}" ;;
    R) cross_region_net_extra_args="${OPTARG}" ;;
    *) usage ;;
  esac
done

if [ -n "$do_create" ];
then
  create_cluster
fi

if [ -n "$do_upload" ];
then
  upload_scripts $CLUSTER
  #load_cockroach $CLUSTER
fi

if [ -n "$do_setup" ];
then
  setup_cluster $CLUSTER
fi

if [ -z "$f_resume" ]
then
  # Execute requested benchmarks.
  for bench in "${benchmarks[@]}"
  do
    $bench
  done
fi

# Wait for benchmarks to finsh and fetch their results.
for bench in "${benchmarks[@]}"
do
  echo "Waiting for $bench to complete"
  fetch="fetch_${bench}_results"
  $fetch
done

if [ -n "$do_destroy" ];
then 
  destroy_cluster
fi
