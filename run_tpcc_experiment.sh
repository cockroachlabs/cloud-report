#!/bin/bash


curdate=$(date '+%Y%m%d')
cloud=
args=

function usage() {
  echo "$1
Usage: $0 -d date
  -d date: date of the folder of the script.
  -c cloud: the script from a specific cloud.
  -a args: arguments for each test script. (e.g. \"-b all -w net -d\")
"
  exit 1
}

while getopts 'a:d:c:' flag; do
  case "${flag}" in
    d) curdate="${OPTARG}" ;;
    c) cloud="${OPTARG}" ;;
    a) args="${OPTARG}" ;;
    *) usage "";;
  esac
done

echo curdate=$curdate

scriptPaths=()

case "${cloud}" in
  aws)
    scriptPaths=($(find ./report-data/$curdate/aws -name "*.sh" |tr '\n' ' '))
    ;;
  gce)
    scriptPaths=($(find ./report-data/$curdate/gce -name "*.sh" |tr '\n' ' '))
    ;;
  azure)
    scriptPaths=($(find ./report-data/$curdate/azure -name "*.sh" |tr '\n' ' '))
    ;;
  *)
    echo "unsupported cloud name"
    exit 1
esac

warehousePerVcpuList=( 50 75 100 125 150 )

# Get the randome string with length 6.
rand_str=$(openssl rand -base64 6)
session_name="run_tpcc_experiment_${cloud}_${rand_str}"
set +e
tmux kill-session -t $session_name
set -e
tmux new -s $session_name -d


for scriptPath in "${scriptPaths[@]}"
do
  for warehousePerVcpu in "${warehousePerVcpuList[@]}"
  do
          diskname="$(basename $(dirname $(dirname "$scriptPath") ))"
          filename=$(basename $scriptPath)

          #echo "diskname:$diskname-$filename-$warehousePerVcpu"
          echo "NAME_EXTRA=$warehousePerVcpu TPCC_WAREHOURSE_PER_VCPU=$warehousePerVcpu $scriptPath -b all -w tpcc -c ./jane-21-5-new-bin "
          tmux neww -t $session_name -n $diskname-$filename-$warehousePerVcpu -d -- "NAME_EXTRA=$warehousePerVcpu TPCC_WAREHOURSE_PER_VCPU=$warehousePerVcpu $scriptPath -b all -w tpcc -c ./jane-21-5-new-bin"
  done
  echo "------"
done