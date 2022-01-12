#!/bin/bash

set -ex
cd netperf

sudo apt-get -y install  automake texinfo librrd-dev libpython3-dev python3-pip net-tools
pip install rrdtool numpy
./autogen.sh
./configure --enable-burst --enable-demo --enable-histogram
sudo make install

sleep 3
echo "finished make install"

cd ..

sudo chmod -R 777 netperf/

export PATH=$PATH:/home/ubuntu/netperf/doc/examples
cd netperf/doc/examples

chmod +x runemomniaggdemo.sh find_max_burst.sh

sudo sysctl -w net.ipv4.tcp_rmem="4096        131072  32000000"
sudo sysctl -w net.ipv4.tcp_wmem="4096        16384   32000000"

f_port=12865
sudo lsof -i :$f_port >/dev/null || sudo netserver -p $f_port



