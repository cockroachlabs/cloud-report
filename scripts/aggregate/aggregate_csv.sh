#!/bin/bash

SOURCE=${BASH_SOURCE%/*}
RESULTS="${SOURCE}/../../results"

echo "uuid,rtt min,rtt avg,rtt max,rtt mdev" > ${RESULTS}/aggregate/network-ping-agg.csv
find ${RESULTS} -name "network-ping.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/network-ping-agg.csv \;

echo "uuid,bogo ops,real time (secs),usr time (secs),sys time (secs),bogo ops/s (real time),bogo ops/s (usr+sys time)" > ${RESULTS}/aggregate/cpu-agg.csv
find ${RESULTS} -name "cpu.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/cpu-agg.csv \;

echo "uuuid,LoadIO (MiB/sec)" > ${RESULTS}/aggregate/io-load-results-agg.csv
find ${RESULTS} -name "io-load-results.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/io-load-results-agg.csv \;

echo "uuid,Threads,Read Throughput,Write Througput,Total Time,Latency Min,Latency Avg,Latency Max,Latency 95th Percentile,Latency Sum" > ${RESULTS}/aggregate/io-rd-results-agg.csv
find ${RESULTS} -name "io-rd-results.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/io-rd-results-agg.csv \;

echo "uuid,Threads,Read Throughput,Write Througput,Total Time,Latency Min,Latency Avg,Latency Max,Latency 95th Percentile,Latency Sum" > ${RESULTS}/aggregate/io-wr-results-agg.csv
find ${RESULTS} -name "io-wr-results.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/io-wr-results-agg.csv \;

echo "uuid,Interval,Transfer,Bandwidth" > ${RESULTS}/aggregate/network-iperf-client-agg.csv
find ${RESULTS} -name "network-iperf-client.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/network-iperf-client-agg.csv \;

echo "uuid,rtt min,rtt avg,rtt max,rtt mdev" > ${RESULTS}/aggregate/network-ping-agg.csv
find ${RESULTS} -name "network-ping.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/network-ping-agg.csv \;

echo "uuid,cloud,machine type,date YYYYMMDD,runID" > ${RESULTS}/aggregate/run-data-agg.csv
find ${RESULTS} -name "run-data.csv" -exec tail -n +2 {} >> ${RESULTS}/aggregate/run-data-agg.csv \;
