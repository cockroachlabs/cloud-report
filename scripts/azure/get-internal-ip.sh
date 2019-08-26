#!/bin/bash

curl -H Metadata:true "http://169.254.169.254/metadata/instance?api-version=2017-08-01" | jq '.network.interface[0].ipv4.ipAddress[0].privateIpAddress'
