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

# We will benchmark *disk* which is mounted as /mnt/data1
#
# !!! WARNING !!!
#    THIS WILL DESTROY DATA ON THE DISK ***
# !!! WARNING !!!
DEV=$(lsblk | grep /mnt/data1|cut -d" " -f1)

# Unmount /mnt/data1 -- the disk we will benchmark; remount when benchmark completes.
sudo umount /mnt/data1
trap "rm -f $pidfile; sudo mkfs.ext4 -F /dev/$DEV && sudo mount /mnt/data1" EXIT SIGINT
echo $$ > "$pidfile"

# Remove processed options.  Remaining ones assumed to be FIO specific flags.
shift $(expr $OPTIND - 1 )

rm -rf "$logdir"
mkdir "$logdir"
report="${logdir}/fio-results.json"
exec &> >(tee -a "$logdir/script.log")

# Dump lsblk and df information (sanity check to make sure we have the right disks)
lsblk
df -h

# Configure FIO with parameters dependent on the type of the disk (SSD vs attached)
case "$DEV" in
  nvme*)
    depth_multiplier=16
    latency_target_ms='10'
    latency_pctl=99
  ;;
  *)
    depth_multiplier=1
    latency_target_ms='1000'
    latency_pctl=95
  ;;
esac

# Bandwidth benchmarks use large (1MB) block size, and run with iodepth_bw depth.
iodepth_bw=$((depth_multiplier * 64))
# IOPs benchmarks use default block size (4KB), and run with very large depth.
iodepth_iops=$((depth_multiplier * 256))
# Latency benchmarks use 4KB block size.  IO depth is small to ensure that
# we do not saturate device bandwidth.  If bandwidth is saturated, latency increases.
iodepth_latency=$((depth_multiplier * 4))

cd "$(dirname $0)"
sudo \
   env IODEPTH_BW=$iodepth_bw IODEPTH_IOPS=$iodepth_iops IODEPTH_LATENCY=$iodepth_latency \
       LATENCY_TARGET="${latency_target_ms}ms" LATENCY_WINDOW="$((latency_target_ms * 10))ms" LATENCY_PCTL=$latency_pctl \
   fio --filename="/dev/$DEV" --output="$report" --output-format=json "$@" ./fio.cfg

touch "$logdir/success"