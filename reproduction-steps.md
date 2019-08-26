## Microbenchmarks

These instructions are an outline of how one would structure a program to automatically run the microbenchmarks for the cloud report. Alternatively, these steps could be done by hand (with slight modifications) or glued together with a big bash script.

1. Create two machines with specified options (cloud, machine type, disk type for AWS) using the steps outlined in `/deployment-steps.md`.
1. Upload scripts to all machines (using the `/scripts` dir in the root of the repo). Make them executable.
1. Execute scripts like this:
    1. **Node 1**: `./scripts/cpu.sh`
    1. **Node 1**: `./scripts/io.sh`
    1. 
        - **Node 2**: `./scripts/network-iperf-server.sh`
        - **Node 1**: `./scripts/network-iperf-client.sh {pgurl:2}`
    1. **Node 1**: `./scripts/network-ping.sh {pgurl:2}`
1. Create directory `/{cloud}/{machinetype}/{dateYYMMDD}/{maxtest#++}/`
1. Only once all scripts have finished, add the following files from each node to the newly created directory:
    - **Node 1**:
        - `cpu.log`
        - `/mnt/data1/IO_LOAD_results.log`
        - `/mnt/data1/IO_WR_results.log`
        - `/mnt/data1/IO_RD_results.log`
        - `network-iperf-client.log`
        - `network-ping.log`
    - **Node 2**:
        - `network-iperf-server.log`

Note that you could loop through any part of this program any number of times you desire; you could run the same tests on the same machine, or run the same tests on multiple sets of machines to get a better understanding of the intrinsic variance in the cloud's VMs.

After this work is done, another process can go through and aggregate the raw data into a format that's easier to digest. That work is TBD.
