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

packages=(
    sysbench make automake libtool
    pkg-config libaio-dev
    libmysqlclient-dev libssl-dev
    libpq-dev cgroup-tools
    fio netperf
    sysstat unzip jq
)

sudo apt-get update
sudo apt-get -y install "${packages[@]}"
chmod 755 cockroach
chown -R ubuntu /mnt/data*

