#!/bin/bash

CLOUD=$1
if [ -z "$CLOUD" ]
then
      echo "error: please specify cloud (gcp, aws, azure) as first arg"
      exit
fi

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

echo "$TEST_FILES_COUNT"

if [ $TEST_FILES_COUNT -ne 64 ]; then
    curl -s https://packagecloud.io/install/repositories/akopytov/sysbench/script.deb.sh | sudo bash

    sudo apt-get -y install sysbench make automake libtool pkg-config libaio-dev libmysqlclient-dev libssl-dev libpq-dev

    if [ "$CLOUD" == "gcp" ]
    then
        sudo umount /mnt/data1
        sudo mount -o discard,defaults,nobarrier $(awk '/\/mnt\/data1/ {print $1}' /etc/mtab) /mnt/data1
    elif [ $CLOUD == "aws" ]
    then
        DEV=$(mount | grep /mnt/data1 | awk '{print $1}'); 
        sudo umount /mnt/data1; 
        sudo mount -o discard,defaults,nobarrier ${DEV} /mnt/data1/; 
        mount | grep /mnt/data1
    elif [ $CLOUD == "azure" ]
    then
        DEV=$(mount | grep /mnt | awk '{print $1}');
        sudo umount /mnt;
        sudo mount -o discard,defaults,barrier=0 ${DEV} /mnt
        mount | grep /mnt
        sudo mkdir /mnt/data1
    else
        echo "Invalid cloud option; choose gcp, aws, or azure" 1>&2
        exit 1
    fi

    sudo chown -R $USER /mnt/data1
    sudo chmod -R 775 /mnt/data1
    cd /mnt/data1
    LIMIT=$((500*1024*1024))
    sudo cgcreate -g memory:group1
    sudo cgset -r memory.limit_in_bytes=$LIMIT group1
    sudo cgset -r  memory.memsw.limit_in_bytes=$LIMIT group1
    sudo cgexec -g memory:group1 sysbench fileio --file-total-size=80G --file-num=64 prepare &> /mnt/data1/io-load-results.log
fi
