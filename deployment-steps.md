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

## EBS gp2

```
roachprod create $CLUSTER -n 4 --clouds=aws --aws-machine-type-ssd=$MACHINETYPE --local-ssd=false
```

## EBS io1

```
roachprod create $CLUSTER -n 4 --clouds=aws --aws-machine-type-ssd=$MACHINETYPE --local-ssd=false --aws-ebs-volume-type=io1 --aws-ebs-iops=20000
```

# Azure

Because `roachprod` doesn't yet support Azure, you must deploy machines manually.

All Azure environments require a manually configured [network with open CRDB ports](https://www.cockroachlabs.com/docs/stable/deploy-cockroachdb-on-microsoft-azure-insecure.html#step-1-configure-your-network).

## 1. Create Machines

### Temporary Disks

#### Creating VM

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

#### Mounting Temporary Disk

SSH to each machine that will become a CockroachDB node and mount the temporary disk using the following commands:

```
mkdir cockroach-data
```
```
sudo mount /dev/sdb1 cockroach-data
```

### Attached Disks

#### Creating VM

**Note**: All machines should be in the same region for these tests.

1. Create 3 VMs with following options:

	Tab | Option | Value
	----|--------|-------
	Basics | Region | Choose the region of your resource group.
	Basics | Resource group | Choose your resource group.
	Basics | Image | Ubuntu Server 18.04 LTS
	Basics | Size | _Variable_
	Disks | Data disks | **Create and attach a new disk** > **1024GiB**
	Networking | Virtual network | Choose the virtual network you configured to open CRDB ports.
	Networking | Select Inbound Ports | SSH

2. Create a 4th VM in the same region as the others, which you'll use to run your workloads.

#### Mounting the Attached Disk

1. Format the remote disk:

	```
	sudo gdisk /dev/sdc
	n
	p
	w
	sudo mkfs -t ext4 /dev/sdc1
	```

2. Mount the attached disk:

	```
	sudo mkdir ~/cockroach-data
	sudo chmod a+rwx ~/cockroach-data
	sudo mount /dev/sdc1 ~/cockroach-data
	```
3. Mount the disk on reboot.
	
	Get the device's UUID from `blkid`
	```
	sudo -i blkid
	```

	Add the device to `/etc/fstab`:
	```
	sudo vim /etc/fstab
	```

	Append a line with the following format:
	```
	UUID=<device UUID>   /datadrive   ext4   defaults,nofail   1   2
	```

	All of this can be combined with the following one-liner:

	```
	sudo -i blkid | grep -Po '\/dev\/sdc1: UUID="\K.{36}' | while read uuid; do sudo echo -e "UUID=${uuid}\t~/cockroach-data\text4\tdefaults,nofail\t1\t2" | sudo tee -a /etc/fstab; done
	```

Note that it is also possible to re-use the same attached disk across VMs by simply detatching it from the first machine and attaching it to subsequent machines. This might be useful with tests that are run serially instead of in parallel.

## 2. Deploying Cockcroach

```
wget -qO- https://binaries.cockroachdb.com/cockroach-v19.1.3.linux-amd64.tgz | tar  xvz
sudo cp -i cockroach-v19.1.3.linux-amd64/cockroach /usr/local/bin
```
```
sudo cockroach start --insecure --advertise-addr=<node1 address> --join=<node1 address>,<node2 address>,<node3 address> --background
```

Initialize the cluster:
```
cockroach init --insecure --host=<address of any node>
```

