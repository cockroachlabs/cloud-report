#!/bin/bash

set -ex
pidfile="$HOME/fio-bench.pid"
f_force=''
f_wait=''

function usage() {
  echo "$1
Usage: $0 [-f] [-w] [-- [fio specific args to override fio.cfg settings]]
  -f: ignore existing pid file; override and rerun.
  -w: wait for currently running benchmark to complete.
"
  exit 1
}
while getopts 'fw' flag; do
  case "${flag}" in
    f) f_flag='true' ;;
    w) f_wait='true' ;;
    *) echo "Usage: $0 [-f] [-w] [-n num_iterations]"
       exit 1 ;;
  esac
done

logdir="$HOME/fio-results"

if [ -n "$f_wait" ];
then
  exec sh -c "
    ( test -f '$logdir/success' ||
      (tail --pid \$(cat $pidfile) -f /dev/null && test -f '$logdir/success')
    ) || (echo 'IO benchmark did not complete successfully.  Check logs'; exit 1)"
fi

if [ -f "$pidfile" ] && [ -z "$f_force" ];
then
  pid=$(cat $pidfile)
  echo "FIO benchmark already running (pid $pid)"
  exit
fi

# Dir specifies directory to store files
# and thus the *disk* to benchmark.
fiodir=/mnt/data1/fio

trap "rm -f $pidfile; sudo rm -rf $fiodir" EXIT SIGINT
echo $$ > "$pidfile"

# Remove processed options.  Remaining ones assumed to be FIO specific flags.
shift $(expr $OPTIND - 1 )

rm -rf "$logdir"
mkdir "$logdir"
report="${logdir}/fio-results.json"
exec &> >(tee -a "$logdir/script.log")

# Dump lsblk and df information (to make sure we have the right disks)
lsblk
df -h

# Uncomment if you want to regenerate test files.
# rm -rf /mnt/data1/fio
sudo mkdir -p "$fiodir"

cd "$(dirname $0)"
sudo cgexec -g memory:group1 fio --output="$report" --output-format=json "$@" ./fio.cfg

touch "$logdir/success"