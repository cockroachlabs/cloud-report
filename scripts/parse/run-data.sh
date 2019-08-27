#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi

RUNDATAPATH="${DIR}/run_data.csv"
DIRASSTRING=${DIR//\// }

echo "uuid,cloud,machine type,date YYYYMMDD,runID" > ${RUNDATAPATH}
UUID=$(cat "${DIR}/uuid.txt")
DATA=$(echo "${DIRASSTRING}" | pcregrep --om-separator="," -o1 -o2 -o3 -o4 'results (.+?) (.+?) (.+?) (.+?)' -)

echo "${UUID},${DATA}" >> ${RUNDATAPATH}
