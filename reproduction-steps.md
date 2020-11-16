# Prep

## Username

If your laptop's username (`whoami`) is not the same as your Cockroach username, please export a global variable (e.g. `.bashrc`) called `CRL_USERNAME`.

This username must also be the same as the username you will use to log into VMs.

## SSH Key

You must use the SSH key in `~/.ssh/id_rsa` to log into any non-`roachprod` VMs, e.g. on-prem machines.

If you need to use a different SSH key, swap it in for your `~/.ssh/id_rsa`.

## Build the Binary

Get the dependencies:

```
go get ./...
```

Build the binary:

```
go build -o cloud-report main.go
```

## Build Roachprod

Go to your `cockroachdb/cockroach` dir.


Make `roachprod`:

```
make bin/roachprod
```

**NOTE**: You must be using this version of `roachprod`, which you can vet is the case with `which roachprod`. If this _isn't_ the version you're using, you should add `$GOPATH/src/github.com/cockroachdb/cockroach/bin` to your `$PATH`.

# Microbenchmarks

## Run on GCP , AWS and Azure

The `cloud-report` binary is responsible for script generation as well as the
analysis of the benchmark results.

When running `cloud-report` binary, specify one or more JSON configuration files
containing the list of the machines to test, as well as their specific configurations
(such as the OS image to use, various arguments to the roachprod, etc).

`./cloudDetails` directory contains configuration files used when running tests
on various cloud providers.

Run `./cloud-report help` to see help on the available commands.

1. Generate scripts to drive benchmarks on each of the configured:
`./cloud-report generate -d ./cloudDetails/aws.json`

`-d ...` option can be repeated multiple times to specify multiple cloud provider
configuration files.

2. The above step will create `./report-data/<date>` directory, with sub-directory
for each of the cloud providers. `./report-data/<date>/<provider>/scripts` directory
contains a shell script, one for each of the machines listed in the cloud providers configuration
file.

3. Execute as many of the generated scripts as necessary.  Each script is almost identical;
   These scripts accept many command line arguments. 
   `./report-data/<date>/<provider>/scripts/machine.sh -b all -w all -d`
    The above will create a cluster for the "machien", do all of the bootstrapping steps
    (`-b all`) and execute all of the benchmarks (`-w all`).  Upon completion, deletes
    the cluster (`-d`).  The script takes many additional arguments to fine tune the
    execution.  See the shell script for details.
 
 4. Results analysis is accomplished via the same program:
   `./cloud-report analyze -d ... -d ...`
   This produces `./report-data/<date>/results/<provider>` directory, with a CSV
   file for each benchmark.
   These files can be imported (google docs, excel, etc) and further analyzed. 