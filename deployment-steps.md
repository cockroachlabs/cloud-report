These instructions detail how we plan to deploy CockroachDB clusters while running tests for the upcoming Cloud Report (Q3 2019).

This document details the plans for each specific cloud, though the only instructions of any real interest are those for Azure.

# GCP

```
roachprod create $CLUSTER -n 4 --gce-machine-type=$MACHINETYPE
```

# AWS

## Default

```
roachprod create $CLUSTER -n 4 --clouds=aws --aws-machine-type-ssd=$MACHINETYPE
```

## EBS io1

```
roachprod create $CLUSTER -n 4 --clouds=aws --aws-machine-type-ssd=$MACHINETYPE --local-ssd=false --aws-ebs-volume-type=io1 --aws-ebs-iops=20000
```

# Azure

Because `roachprod` doesn't yet support Azure, you must deploy machines manually.

All Azure environments require a manually configured [network with open CRDB ports](https://www.cockroachlabs.com/docs/stable/deploy-cockroachdb-on-microsoft-azure-insecure.html#step-1-configure-your-network).

## 1. Create Machines

### Creating VM

**Note**: All machines should be in the same region for these tests.

1. Create 3 VMs with following options:

	Tab | Option | Value
	----|--------|-------
	Basics | Region | Choose the region of your resource group.
	Basics | Resource group | Choose your resource group.
	Basics | Image | Ubuntu Server 18.04 LTS
	Basics | Size | _Variable_
	Networking | Virtual network | Choose the virtual network you configured to open CRDB ports.
	Networking | Select Inbound Ports | SSH

	You must make sure that the machine's temp disk is >127GiB.

2. Create a 4th VM in the same region as the others, which you'll use to run your workloads.

### Mounting Temporary Disk

SSH to each machine that will become a CockroachDB node and mount the temporary disk using the following commands:

```
mkdir cockroach-data
```
```
sudo mount /dev/sdb1 cockroach-data
```

## 2. Deploying Cockcroach

Get the Cockroach binary.
```
wget -qO- https://binaries.cockroachdb.com/cockroach-v19.1.3.linux-amd64.tgz | tar  xvz
sudo cp -i cockroach-v19.1.3.linux-amd64/cockroach /usr/local/bin
```

Enable `nobarrier`.
```
DEV=$(mount | grep /mnt | awk '{print $1}');
sudo umount /mnt;
sudo mount -o discard,defaults,barrier=0 ${DEV} /mnt
mount | grep /mnt
sudo mkdir /mnt/data1
```

Start the node:
```
sudo cockroach start --insecure --advertise-addr=<node1 address> --join=<node1 address>,<node2 address>,<node3 address> --store=/mnt/data1 --background
```

After starting the node on all machines, initialize the cluster:
```
cockroach init --insecure --host=<address of any node>
```
