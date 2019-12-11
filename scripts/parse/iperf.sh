#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi

IPERFLOGPATH="${DIR}/network-iperf-server.log"
IPERFCSVPATH="${DIR}/network-iperf-server.csv"

if ! [ -f "$IPERFLOGPATH" ]; then
    echo "$IPERFLOGPATH does not exist"
    exit
fi

echo "uuid,Interval,Transfer,Bandwidth" > ${IPERFCSVPATH}
UUID=$(cat "${DIR}/uuid.txt")
for LINE in 1  2 3; do
  DATA=$(tail -n ${LINE} ${IPERFLOGPATH} | head -n 1 | pcregrep --om-separator="," -o1 -o2 -o3 '\]\s+(.+?)\s+sec\s+(.+?)Bytes\s+(.+? .+?)bits' -);
  echo "${UUID},${DATA}" >> ${IPERFCSVPATH}
done;
