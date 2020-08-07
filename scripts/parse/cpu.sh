#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi
CPULOGPATH="${DIR}/cpu.log"
CPUCSVPATH="${DIR}/cpu.csv"

if ! [ -f "$CPULOGPATH" ]; then
    echo "$CPULOGPATH does not exist"
    exit
fi

echo "uuid,bogo ops,real time (secs),usr time (secs),sys time (secs),bogo ops/s (real time), bogo ops/s (usr+sys time)" > ${CPUCSVPATH}

UUID=$(cat "${DIR}/uuid.txt")
DATA=$(pcregrep --om-separator="," -o1 -o2 -o3 -o4 -o5 -o6 'matrix\s+(\d+?)\s+(.+?)\s+(.+?)\s+(.+?)\s+(.+?)\s+(.+?)$' ${CPULOGPATH})

echo "${UUID},${DATA}" >> ${CPUCSVPATH}
