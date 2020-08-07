#!/bin/bash

TEST_FILES_COUNT=0

for f in /mnt/data1/test_file*; do

    ## Check if the glob gets expanded to existing files.
    ## If not, f here will be exactly the pattern above
    ## and the exists test will evaluate to false.
    if [ -e "$f" ]; then
        TEST_FILES_COUNT=$(ls -dq /mnt/data1/test_file* | wc -l )
    fi
    break
done

if [ $TEST_FILES_COUNT -ne 64 ]; then
    echo "Missing test files" > /mnt/data1/io-wr-results.log
    exit
fi

cd /mnt/data1

> /mnt/data1/io-rd-results.log

# Set memory to 50 Megabytes
for each in 1 4 8 16 32 64
do 
  sudo cgexec -g memory:group1   \
    sysbench fileio              \
    --file-total-size=80G        \
    --file-test-mode=rndrd       \
    --time=240                   \
    --max-requests=0             \
    --file-block-size=32K        \
    --file-num=64                \
    --file-fsync-all             \
    --file-fsync-mode=fdatasync  \
    --threads=$each run

  sleep 10;
done &>> /mnt/data1/io-rd-results.log

