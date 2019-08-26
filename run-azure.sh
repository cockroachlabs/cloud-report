#!/bin/bash

nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-D16s-v3.txt
nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-DS5-v2.txt
nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-F16s-v2.txt
nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-E16s-v3.txt
nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-DS14-v2.txt
nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-GS4.txt
nohup ./cloud-report-2019 -azure -node1=<> -node2=<> -iterations=2 &
echo $! > logs/azure/pid-H16r.txt
