#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi
IOLOADLOGPATH="${DIR}/io-load-results.log"
IOLOADCSVPATH="${DIR}/io-load-results.csv"

if ! [ -f "$IOLOADLOGPATH" ]; then
    echo "$IOLOADLOGPATH does not exist"
    exit
fi

echo "uuid,LoadIO (MiB/sec)" > ${IOLOADCSVPATH}
UUID=$(cat "${DIR}/uuid.txt")
DATA=$(pcregrep --om-separator="," -o1 '\((\d+\.\d+)\sMiB\/sec\)' ${IOLOADLOGPATH})
echo "${UUID},${DATA}" >> ${IOLOADCSVPATH}
