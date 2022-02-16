#!/opt/homebrew/bin/bash

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

while getopts 'a:d:c:' flag; do
  case "${flag}" in
    d) curdate="${OPTARG}" ;;
    c) cloud="${OPTARG}" ;;
    a) args="${OPTARG}" ;;
    *) usage "";;
  esac
done

if [ -z "CRL_USERNAME" ]
  export CRL_USERNAME=$USER
fi

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

warehousePerVcpuList=( 38 50 75 100 125 150 )

function prompt() {
  if [ -n "$yes_all" ]
  then
    return 0
  fi

  echo -n "$@ [y|Y|n]? "
  local answer=''
  read answer
  case "${answer}" in
    y) return 0 ;;
    Y) yes_all='y'
       return 0
      ;;
    *) return 1 ;;
  esac 
}


for scriptPath in "${scriptPaths[@]}"
do
  if  ! prompt "Execute $scriptPath" 
  then
    echo "Skipping ..."
    continue
  fi

  session_name="${cloud}_`basename $scriptPath .sh`"
  set +e
  tmux new -s $session_name -d
  set -e
 
  for warehousePerVcpu in "${warehousePerVcpuList[@]}"
  do
    if  ! prompt "Execute $scriptPath vcpu=$warehousePerVcpu" 
    then
      echo "Skipping ..."
      continue
    fi

    for run in 1 2 3
    do
      diskname="$(basename $(dirname $(dirname "$scriptPath") ))"
      filename=$(basename $scriptPath)
   
      name_extra="$warehousePerVcpu-$run"
      echo "tmux neww -t $session_name -n $diskname-$filename-$name_extra -d -- \"NAME_EXTRA=$name_extra TPCC_EXTRA_ARGS='-a $warehousePerVcpu'  $scriptPath -b all -w tpcc -c ./cockroach-linux-2.6.32-gnu-amd64 -d\""
      tmux neww -t $session_name -n $diskname-$filename-$name_extra -d -- "NAME_EXTRA=$name_extra TPCC_EXTRA_ARGS='-a $warehousePerVcpu' $scriptPath -b all -w tpcc -c ./cockroach-linux-2.6.32-gnu-amd64 -d"
      echo "Sleeping for 10s"
      sleep 10
    done
  done

  echo "------"
done
