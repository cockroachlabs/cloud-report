#!/bin/bash

set -ex
pidfile="$HOME/cpu-bench.pid"
f_force=''
f_wait=''
f_iters=10

while getopts 'fwn:' flag; do
  case "${flag}" in
    f) f_flag='true' ;;
    n) f_iters="${OPTARG}" ;;
    w) f_wait='true' ;;
    *) echo "Usage: $0 [-f] [-w] [-n num_iterations]"
       exit 1 ;;
  esac
done

logdir="$HOME/coremark-results"

if [ -n "$f_wait" ];
then
  exec sh -c "
    ( test -f '$logdir/success' ||
      (tail --pid \$(cat $pidfile) -f /dev/null && test -f '$logdir/success')
    ) || (echo 'CPU benchmark did not complete successfully.  Check logs'; exit 1)"
fi

if [ -f "$pidfile" ] && [ -z "$f_force" ] ;
then
  pid=$(cat $pidfile)
  echo "CPU benchmark already running (pid $pid)"
  exit
fi

trap "rm -f $pidfile" EXIT SIGINT
echo $$ > "$pidfile"

rm -rf "$logdir"
mkdir "$logdir"

exec &> >(tee "$logdir/script.log")

if [ ! -d coremark ]
then
  git clone https://github.com/eembc/coremark.git
fi

cd coremark

# Dump CPU info into logs (sanity check)
cat /proc/cpuinfo

# Build default coremark (single proc)
make REBUILD=1 link

for ((i=0; i < f_iters; i++))
do
  echo "Single core iteration: $i"
  ./coremark.exe > "${logdir}/single-$i.log"
done


# Rebuild to run in multithreaded mode.
make LFLAGS_END="-lpthread" XCFLAGS="-DMULTITHREAD=$(nproc) -DUSE_PTHREAD" REBUILD=1 link
for ((i=0; i < f_iters; i++))
do
  echo "Multi core iteration: $i"
  ./coremark.exe > "${logdir}/multi-$i.log"
done

touch "$logdir/success"