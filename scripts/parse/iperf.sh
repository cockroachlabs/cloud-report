#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi

IPERFLOGPATH="${DIR}/network-iperf-client.log"
IPERFCSVPATH="${DIR}/network-iperf-client.csv"

if ! [ -f "$IPERFLOGPATH" ]; then
    echo "$IPERFLOGPATH does not exist"
    exit
fi

echo "uuid,Interval,Transfer,Bandwidth" > ${IPERFCSVPATH}
UUID=$(cat "${DIR}/uuid.txt")
DATA=$(tail -n 1 ${IPERFLOGPATH} | pcregrep --om-separator="," -o1 -o2 -o3 '\]\s+(.+?)\s+sec\s+(.+?)Bytes\s+(.+? .+?)bits' -)
echo "${UUID},${DATA}" >> ${IPERFCSVPATH}
