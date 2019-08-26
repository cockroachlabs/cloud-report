#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi

PINGLOGPATH="${DIR}/network-ping.log"
PINGCSVPATH="${DIR}/network-ping.csv"

if ! [ -f "$PINGLOGPATH" ]; then
    echo "$PINGLOGPATH does not exist"
    exit
fi

echo "uuid,rtt min,rtt avg,rtt max,rtt mdev" > ${PINGCSVPATH}
UUID=$(cat "${DIR}/uuid.txt")
DATA=$(pcregrep --om-separator="," -o1 -o2 -o3 -o4 'rtt min/avg/max/mdev = (.+?)\/(.+?)\/(.+?)\/(.+?) ' ${PINGLOGPATH})
echo "${UUID},${DATA}"  >> ${PINGCSVPATH}
