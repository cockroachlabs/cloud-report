#!/bin/bash

# Dir specifies directory to store files
# and thus the *disk* to benchmark.
fiodir=/mnt/data1/fio
mkdir -p "$fiodir"

fioargs=(
  --rw=randrw        # Random read/write
  --rwmixread=80     # 80% reads
  --direct=1         # Direct IO; no buffering
  --iodepth=1        # Sync every op.  This matters for databases.
  --end_fsync=1
  --fsync_on_close=1
  --size=512M        # Input size
  --ioengine=libaio
  --runtime=30       # Benchmark duration
  --directory="$fiodir"
)

(
for bs in 4k 8k 16k
do
  for j in 1 4 8 16
  do
    echo "Running fio: jobs=$j bs=$bs"
    fio --name="randrw-$bs" --bs=$bs --numjobs=$j "${fioargs[@]}"
  done
done
) | tee "$fiodir/fio-report.log"
