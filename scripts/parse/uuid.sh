#!/bin/bash

# This file is meant to be invoked by parse-dir.sh.

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi

UUIDPATH="${DIR}/uuid.txt"

if ! [ -f "$UUIDPATH" ]; then
    uuidgen > ${UUIDPATH}
fi
