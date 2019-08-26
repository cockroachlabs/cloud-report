- `README.md`: High-level overview.
- `cloudDetails`: Contains `.json` files that describe the cloud providers and VMs to run the benchmarks on. Is unmarshalled into `[]CloudDetails`.
- `deployment-steps.md`: Instructions for provisioning VMs, or a high-level overview for what we expect to do via `roachprod`.
- `googleSheets.go`: Contains all of the code to actually post results to Google Sheets.
- `init.sh`: An initialization script placed on machines to prep them for the benchmarks. Because it contains instructions to unzip the `scripts` dir, it should be left outside the `scripts` dir.
- `logs`: Initialization and run logs of the VMs. Automtatically populated by the binary.
    - Structured as `logs/<cloud>/<machine type>/<YYYMMDD>/<run|init>/<run ID>`
- `main.go`: Primary implementation of the binary.
- `reproduction-steps.md`: Directions for using the binary to generate results.
- `results`: The results of the benchmark runs collected from the machines.
    - Structured as `results/<cloud>/<machine type>/<YYYMMDD>/<run ID>`
    - `results/aggregate` contains a concatenation of all CSVs in the `results/<cloud>` dirs.
- `run-azure.sh`: A cludgy script to parallelize running on Azure. Generates a bunch of `.txt` files to track the PIDs of the running processes.
- `scripts`: 
    - `aggregate`: `aggregate_csv.sh` takes all of the CSVs in the `results/<cloud>` dirs, aggregates them, and places them in `results/aggregate`
    - `azure`: Contains scripts to run on Azure machines to collect metadata about the VMs. This will be obviated by running Azure in `roachprod`, but provides a useful template for any platform we want to use that is ever outside of `roachprod`.
    - `gen`: Contains scripts that actually run the benchmarks and generate output.
    - `parse`: Contains scripts that convert raw output from the benchmarks and converts them into CSVs.

**Note:** For the binary to work, you should also have a `credentials.json` and `token.json` file in the root directory of this project.
