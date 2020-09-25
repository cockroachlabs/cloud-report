#!/bin/bash

#
# Execute this script on every node in the cluster
# to configure and install required packages.
# This script is intended to be invoked via "roachprod" command.
# Alternatively, execute this script manually.

exec &> >(tee -a "setup.log")

if [ "$EUID" != 0 ]
then
  echo "error: this script must execute under sudo"
  exit 1
fi

CLOUD=$1
if [ -z "$CLOUD" ]
then
      echo "error: please specify cloud (gcp, aws, azure) as first arg"
      exit
fi

packages=(
    sysbench make automake libtool
    pkg-config libaio-dev
    libmysqlclient-dev libssl-dev
    libpq-dev cgroup-tools
    fio netperf
    unzip jq
)

apt-get update
apt-get -y install "${packages[@]}"
bash -c "$(wget -O - https://apt.llvm.org/llvm.sh)"
curl -s https://packagecloud.io/install/repositories/akopytov/sysbench/script.deb.sh | sudo bash

if [ "$CLOUD" == "gcp" ]
then
    umount /mnt/data1
    mount -o discard,defaults "$(awk '/\/mnt\/data1/ {print $1}' /etc/fstab)" /mnt/data1
elif [ "$CLOUD" == "aws" ]
then
    DEV=$(mount | grep /mnt/data1 | awk '{print $1}');
    umount /mnt/data1
    mount -o discard,defaults "${DEV}" /mnt/data1/;
    mount | grep /mnt/data1
elif [ "$CLOUD" == "azure" ]
then
    DEV=$(mount | grep /mnt | awk '{print $1}');
    umount /mnt;
    mount -o discard,defaults "${DEV}" /mnt
    mount | grep /mnt
    mkdir -p /mnt/data1
else
    echo "Invalid cloud option; choose gcp, aws, or azure" 1>&2
    exit 1
fi

chown -R "$USER" /mnt/data1
chmod -R 775 /mnt/data1
cd /mnt/data1
LIMIT=$((500*1024*1024))
cgcreate -g memory:group1
cgset -r memory.limit_in_bytes=$LIMIT group1
cgset -r  memory.memsw.limit_in_bytes=$LIMIT group1
mkdir /cgroup
mount -t cgroup -o blkio none /cgroup

