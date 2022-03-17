#!/bin/bash

set -ex
pidfile="$HOME/fio-bench.pid"
f_force=''
f_wait=''
f_ssd=''
f_cfg='fio.cfg'
function usage() {
  echo "$1
Usage: $0 [-f] [-w] [-s] [-c fio.cfg] [-- [fio specific args to override fio.cfg settings]]
  -f: ignore existing pid file; override and rerun.
  -w: wait for currently running benchmark to complete.
  -s: assume disk is local SSD
  -c: FIO config
"
  exit 1
}
while getopts 'fwsc:' flag; do
  case "${flag}" in
    f) f_flag='true' ;;
    w) f_wait='true' ;;
    s) f_ssd='true' ;;
    c) f_cfg="${OPTARG}" ;;
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
if mountpoint -q /mnt/data1;
then
  mount=$(readlink /mnt/data1 || echo /mnt/data1)
else
  mount="/mnt"
fi
DEV=$(lsblk -r | grep $mount|cut -d" " -f1)

function restore_fs() {
  sudo mkfs.ext4 -F /dev/$DEV
  sudo mount $mount
  sudo mkdir $mount/cockroach
  sudo chown ubuntu $mount/cockroach
  rm $pidfile
}

# We run 8 bandwidth jobs -- have each operation on different portion
# of the disk by specifying offset increment == 1/8 of disk size (in MB)
OFFSET="$(lsblk  -rb|grep /mnt/data1|awk '{print int($4/2^23)}')M"

# Unmount /mnt/data1 -- the disk we will benchmark; remount when benchmark completes.
sudo umount "$mount"
trap restore_fs EXIT SIGINT
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
depth_multiplier=1
if [ -n "$f_ssd" ]; then
  depth_multiplier=16
fi

# Bandwidth benchmarks use large (1MB) block size, and run with iodepth_bw depth.
iodepth_bw=$((depth_multiplier * 64))
# IOPs benchmarks use default block size (4KB), and run with very large depth.
iodepth_iops=$((depth_multiplier * 512))
# Latency benchmarks use 4KB block size.  IO depth is small to ensure that
# we do not saturate device bandwidth.  If bandwidth is saturated, latency increases.
iodepth_latency=$((depth_multiplier * 16))

cd "$(dirname $0)"
sudo \
   env IODEPTH_BW=$iodepth_bw IODEPTH_IOPS=$iodepth_iops IODEPTH_LATENCY=$iodepth_latency \
       OFFSET="$OFFSET" \
   fio --filename="/dev/$DEV" --output="$report" --output-format=json "$@" "${f_cfg}"

touch "$logdir/success"
