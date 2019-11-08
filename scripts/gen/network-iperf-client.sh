#!/bin/bash

SERVER=$1
if [ -z "$SERVER" ]
then
      echo "error: please specify internal IP of server"
      exit
fi

sudo apt-get install -y iperf nmap
sleep 5
nmap -p 5001 $SERVER | grep tcp &> nmap.log
iperf --client="$SERVER" --len=128k --interval=1 --time=60 &> network-iperf-client.log
if [ $? -ne 0 ]; then
    exit 1
fi
sleep 15