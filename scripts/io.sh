#!/bin/bash

CLOUD=$1
if [ -z "$CLOUD" ]
then
      echo "error: please specify cloud (gcp, aws, azure) as first arg"
      exit
fi

curl -s https://packagecloud.io/install/repositories/akopytov/sysbench/script.deb.sh | sudo bash
sudo apt-get update
sudo apt -y install sysbench make automake libtool pkg-config libaio-dev libmysqlclient-dev libssl-dev libpq-dev

if [ "$CLOUD" == "gcp" ]
then
    sudo umount /mnt/data1
    sudo mount -o discard,defaults,nobarrier $(awk '/\/mnt\/data1/ {print $1}' /etc/mtab) /mnt/data1
elif [ $CLOUD == "aws" ]
then
    sudo umount /mnt/data1; sudo mount -o discard,defaults,nobarrier /dev/nvme1n1 /mnt/data1/; mount | grep /mnt/data1
else
    echo "No Azure steps yet"
fi

sudo chmod u+w /mnt/data1
cd /mnt/data1
sysbench fileio --file-total-size=8G --file-num=64 prepare > /mnt/data1/IO_LOAD_results.log

for each in 1 4 8 16 32 64; do sysbench fileio --file-total-size=8G --file-test-mode=rndwr --time=240 --max-requests=0 --file-block-size=32K --file-num=64 --file-fsync-all --threads=$each run; sleep 10;
done > /mnt/data1/IO_WR_results.log

for each in 1 4 8 16 32 64; do sysbench fileio --file-total-size=8G --file-test-mode=rndrd --time=240 --max-requests=0 --file-block-size=32K --file-num=64 --file-fsync-all --threads=$each run; sleep 10;
done > /mnt/data1/IO_RD_results.log
