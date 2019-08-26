#!/bin/bash

SERVER=$1
if [ -z "$SERVER" ]
then
      echo "error: please specify internal IP of server"
      exit
fi

sudo apt-get update
sudo apt-get install iperf -y
ip a
iperf --client=$SERVER --len=128k --interval=1 --time=60 > network-iperf-client.log
