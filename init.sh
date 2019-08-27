#!/bin/bash

sudo apt-get update -y
sudo apt-get install -y unzip jq
unzip -o scripts.zip
chmod -R a+x scripts
