#!/bin/bash

CLOUD=$1
if [ -z "$CLOUD" ]
then
      echo "error: please specify cloud (gcp, aws, azure) as first arg"
      exit
fi

curl -s https://packagecloud.io/install/repositories/akopytov/sysbench/script.deb.sh | sudo bash

sudo apt-get -y install sysbench make automake libtool pkg-config libaio-dev libmysqlclient-dev libssl-dev libpq-dev

if [ "$CLOUD" == "gcp" ]
then
    sudo umount /mnt/data1
    sudo mount -o discard,defaults,nobarrier $(awk '/\/mnt\/data1/ {print $1}' /etc/mtab) /mnt/data1
elif [ $CLOUD == "aws" ]
then
    sudo umount /mnt/data1
    sudo mount -o discard,defaults,nobarrier /dev/nvme1n1 /mnt/data1/
    mount | grep /mnt/data1
elif [ $CLOUD == "azure" ]
then
    sgdisk -n 1:2048:2145386462
    yes | sudo mkfs -t ext4 /dev/sdc1
    sudo mount -o discard,defaults,barrier=0 /dev/sdc1 /mnt/data1
    mount | grep /mnt/data1
else
    echo "Invalid cloud option; choose gcp, aws, or azure" 1>&2
    exit 1
fi

sudo chown -R $USER /mnt/data1
sudo chmod -R 775 /mnt/data1
cd /mnt/data1
sysbench fileio --file-total-size=8G --file-num=64 prepare &> /mnt/data1/IO_LOAD_results.log

for each in 1 4 8 16 32 64; do sysbench fileio --file-total-size=8G --file-test-mode=rndwr --time=240 --max-requests=0 --file-block-size=32K --file-num=64 --file-fsync-all --threads=$each run; sleep 10;
done &>> /mnt/data1/IO_WR_results.log

for each in 1 4 8 16 32 64; do sysbench fileio --file-total-size=8G --file-test-mode=rndrd --time=240 --max-requests=0 --file-block-size=32K --file-num=64 --file-fsync-all --threads=$each run; sleep 10;
done &>> /mnt/data1/IO_RD_results.log
