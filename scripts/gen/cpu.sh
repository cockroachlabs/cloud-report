#!/bin/bash

sudo apt-get install stress-ng -y
stress-ng --metrics-brief --matrix=16 --timeout=1m &> cpu.log
