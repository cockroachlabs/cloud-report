# Prep

## Username

If your laptop's username (`whoami`) is not the same as your Cockroach username, please export a global variable (e.g. `.bashrc`) called `CRL_USERNAME`.

This username must also be the same as the username you will use to log into VMs.

## SSH Key

You must use the SSH key in `~/.ssh/id_rsa` to log into any non-`roachprod` VMs, e.g. on-prem machines.

If you need to use a different SSH key, swap it in for your `~/.ssh/id_rsa`.

## Setting up Google Drive Access

Unless you're running the enclosed binary through the `-on-prem` option, it relies on Google Drive access to automatically post its results to a spreadsheet.

For this to work, you must set up Google Drive Access:

1. Go to the [API console](https://console.developers.google.com/).
1. Switch to your **@cockroachlabs.com** account, and the **cockroach-shared** project.
1. Click **+Enable APIs and Services**.
1. Find the **Google Sheets API** link and click it.
1. Click **Manage**.
1. Click **Credentials** in the sidebar.
1. Click **Download** next tp the **Cloud Report** user under **OAuth 2.0 Clients**.
1. Rename the downloaded file to `credentials.json` and move it into the root directory of this repo.
1. Now when you launch the benchmark for the first time and get to a spot where you need to upload the results to Google Sheets, the CLI will guide you through providing access to it.

## Build the Binary

Get the dependencies:

```
go get ./...
```

Build the binary:

```
go build
```

## Build Roachprod

Go to your `cockroachdb/cockroach` dir.


Make `roachprod`:

```
make bin/roachprod
```

**NOTE**: You must be using this version of `roachprod`, which you can vet is the case with `which roachprod`. If this _isn't_ the version you're using, you should add `$GOPATH/src/github.com/cockroachdb/cockroach/bin` to your `$PATH`.

# Microbenchmarks

## Run on GCP & AWS

To test all machines, run:
~~~
./cloud-report-2019
~~~

Meaningful flags include:

** Flag ** | ** Operation **
-----------|----------------
`-cloud-details` | Specify a JSON file to detail the machine types you want to test. Use `cloudDetails/default.json` as a template. <br/><br/>For any machines you want to test with EBS on AWS, make sure they're listed as `ebsMachineTypes`.
`-io-skip` | Skip the IO tests, which take a long time to complete
`-iterations` | Run the benchmark tests _x_ times against the same machines. To run the tests against a separate set of machines, you must manually destroy the roachprod cluster that gets created.

As noted above, you can choose some other set of machines to test by specifying another file with `-cloud-details`.

## Run on Azure

1. Manually provision the machines you want to test on Azure, with a few crucial considerations:

    - The user name must match `whoami`
    - The SSH key must be `~/.ssh/id_rsa`
    - `-node2` should be in the same zone as `-node1` but doesn't necessarily need to be the same machine type.
    
2. Run...

    ~~~
    ./cloud-report-2019 -azure -node1 <public IP of node 1> -node2 <public IP of node 2>
    ~~~

Meaningful flags include:

** Flag ** | ** Operation **
-----------|----------------
`-io-skip` | Skip the IO tests, which take a long time to complete
`-iterations` | Run the benchmark tests _x_ times against the same machines.

## Run on Arbitrary Platforms/On-Prem

**NOTE**: This method _does not_ persist any results besides to your local machine. Or, phrased another way, these results are not stored to Google Sheets.

1. Manually provision the machines you want to test, with a few crucial considerations:

    - The SSH key must be `~/.ssh/id_rsa`
    - `-node2` will work as a server for `-node1`, so should be on the same subnet. However, you can override this behavior by specifying `-node2-internal` to any IP address you'd like.

2. Run...
    ```
    ./cloud-report-2019 -on-prem -node1 <public IP of node 1> -node2 <public IP of node 2>
    ```

Meaningful flags include:

** Flag ** | ** Operation **
-----------|----------------
`-io-skip` | Skip the IO tests, which take a long time to complete
`-iterations` | Run the benchmark tests _x_ times against the same machines.
`machine-type` | Store the results of this run under `results/on-prem/<machine-type>`; if not set, stores results in `results/on-prem/<node1-ip>`
`-node2-internal` | If we can't automatically detect the second node's internal IP address, you might need to pass it in manually.

# Results

Results for each benchmark are automatically saved and parsed into CSVs in the `results` folder with the following structure:

`results/<platform>/<machine type>/<date>/<run id>`

At the end of results, there will be `.log` files for all completed benchmarks, and `.csv` files for everything that was successfully parsed (and potentially uploaded). If there are no `.csv` files, the results were not parsed and were definitely not uploaded.

### Aggregate

Though it currently has no use or purpose, you can aggregate all of the results in the `results` dir by running the script in `scripts/aggregate`. Note that this should roughly match what is stored on Google Sheets and should make it easier to move this data into a database (e.g. Cockroach).
