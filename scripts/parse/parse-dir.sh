#!/bin/bash

DIR=$1
if [ -z "$DIR" ]
then
      echo "error: please specify directory to parse"
      exit
fi

SOURCE=${BASH_SOURCE%/*}

${SOURCE}/uuid.sh ${DIR}
${SOURCE}/run-data.sh ${DIR}
${SOURCE}/cpu.sh ${DIR}
${SOURCE}/iperf.sh ${DIR}
${SOURCE}/ping.sh ${DIR}
${SOURCE}/io-load.sh ${DIR}
${SOURCE}/io-rdwr.sh ${DIR} "R"
${SOURCE}/io-rdwr.sh ${DIR} "W"
