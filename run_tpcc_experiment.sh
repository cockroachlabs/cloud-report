#!/bin/bash

# bash version should be > 5

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

while getopts 'a:d:c:n' flag; do
  case "${flag}" in
    n) dry_run="y" ;;
    d) curdate="${OPTARG}" ;;
    c) cloud="${OPTARG}" ;;
    a) args="${OPTARG}" ;;
    *) usage "";;
  esac
done

echo curdate=$curdate


# The rest of the arguments may contain paths to the scripts to execute.
# If not specified, attempts to find scripts in the cloud directory.
shift $((OPTIND - 1 ))
scriptPaths=("$@")

if [[ ${#scriptPaths[@]} == 0 ]]
then
  case "${cloud}" in
    aws)
      mapfile -t scriptPaths < <( find ./report-data/$curdate/aws -name "*.sh" )
      ;;
    gce)
      mapfile -t scriptPaths < <( find ./report-data/$curdate/gce -name "*.sh" |grep standard|grep -v 30 )
      ;;
    *)
      echo "unsupported cloud name"
      exit 1
  esac
fi


num_runs_per_vcpu=4

function prompt() {
  num_runs_per_vcpu=4

  if [ -n "$yes_all" ]
  then
    return 0
  fi

  echo -n "$@ [y|Y|#|n]? "
  local answer=''
  read answer

  case "${answer}" in
    y) return 0 ;;
    Y) yes_all='y'
       return 0
      ;;
    1|2|3|4|5)
       num_runs_per_vcpu=$answer
       return 0
      ;;
    *) return 1 ;;
  esac 
}


for scriptPath in "${scriptPaths[@]}"
do
  if  ! prompt "Execute '$scriptPath'" 
  then
    echo "Skipping ..."
    continue
  fi

  session_name="${cloud}_`basename $scriptPath .sh`"
  set +e
  tmux new -s $session_name -d
  set -e

  if [[ $scriptPath =~ 32|30|8xl|9x ]]
  then
    warehousePerVcpuList=( 50 75 100 125 150 1200 )
  else 
    warehousePerVcpuList=( 50 75 100 125 150 )
  fi

  for warehousePerVcpu in "${warehousePerVcpuList[@]}"
  do
    if  ! prompt "Execute '$scriptPath' vcpu=$warehousePerVcpu" 
    then
      echo "Skipping ..."
      continue
    fi

    for (( run=1; run<=$num_runs_per_vcpu; run++ ))
    do
      diskname="$(basename $(dirname $(dirname "$scriptPath") ))"
      filename=$(basename $scriptPath)
  
      if (( warehousePerVcpu < 1000 ))
      then
        extra_arg="-a $warehousePerVcpu"
      else
        extra_arg="-A $warehousePerVcpu"
      fi

      name_extra="$warehousePerVcpu-$run"
      echo "tmux neww -t $session_name -n $diskname-$filename-$name_extra -d -- \"CRL_USERNAME=$USER NAME_EXTRA=$name_extra TPCC_EXTRA_ARGS='$extra_arg'  $scriptPath -b all -w tpcc -c ./cockroach-linux-2.6.32-gnu-amd64 -d\""

      if [ -z "$dry_run" ]
      then
        tmux neww -t $session_name -n $diskname-$filename-$name_extra -d -- "CRL_USERNAME=$USER NAME_EXTRA=$name_extra TPCC_EXTRA_ARGS='$extra_arg' $scriptPath -b all -w tpcc -c ./cockroach-linux-2.6.32-gnu-amd64 -d"
        echo "Sleeping for 30s"
        sleep 30
      fi
    done
  done

  #echo "Sleeping for 10 minutes"
  #echo sleep 600
  echo "------"
done
