#!/bin/bash

sudo apt-get update
sudo apt-get install stress-ng -y
stress-ng --metrics-brief --cpu=16 --timeout=1m > cpu.log
