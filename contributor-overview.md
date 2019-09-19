# Program Structure

In short, the enclosed binary:

1. _In some circumstances_<sup>1</sup>, provisions machines.
1. Places scripts in `scripts/gen` onto remote machines, and executes them.
1. Downloads the scripts' output to `results`.
1. Converts the output into CSV.
1. _In some circumstances_<sup>2</sup>, posts the results to Google Sheets.

<sup>1</sup>: When running on `roachprod`-compatible platform, you can have `roachprod` create and manage the VMs for you.

<sup>2</sup>: When running with the `on-prem` flag, results are not stored in Google Sheets.

## Adding Scripts

### New Benchmarks

1. Place a `.sh` script in `scripts/gen` that executes the benchmark you want, and stores its output (through output redirection, i.e. `>`). Whatever you name this will be the "artifact" you end up pulling down and parsing when the benchmark completes.
1. Create a script that parses the raw output of the benchmark into a complete CSV file, i.e. with a header, and should place the run's UUID as the first element. Add this script to `scripts/parse`. And then add a call to this script to `scripts/parse/parse-dir.sh`.
  For inspiration, check any of the existing scripts in `scripts/parse`. The initial scripts rely heavily on `pcregrep`--sorry.
1. Add a new sheet to the tracking spreadsheet and identify the range where the values should be posted. Add this range so that it matches the format found in `filenameToSSRange` in `googleSheets.go`.
1. Add a new `benchmark` to the global `benchmarks` struct in `main.go` that references this file as a `benchmarkRoutine`. Identify the output in `benchmark.artifact`.
    - If the benchmark requires an argument, check out `argConstArr` in `main.go` to see if it already exists. If not, please add the argument you need as a const with the other args (e.g. `argCloudName`), and add it to `argConstArr`. You can specify arguments within each `benchmarkRoutine`.

## Pseduo-FAQ

**How does this binary work?**

It heavily relies on shelling out to bash scripts.

**What's up with passing an *os.File to every function?**

I wanted to parallelize the execution of all of the `roachprod`-compatible runs, but I wanted to keep the high-fidelity logging I had for individual runs. To handle this, everything logs to some individual file that gets instantiated in one of the `<platform>Run()` functions. I'm sure there's a better way to do this, but it was the path of least resistance at the time.

**Why can't I successfully build `n2-` class machines in GCP?**

See [roachprod: Add provisioning of GCP n2-class machines w/ local SSDs](https://github.com/cockroachdb/cockroach/pull/40801). Once that's merged, your `roachprod` builds need to include this commit.

# TOC

- `README.md`: High-level overview.
- `cloudDetails`: Contains `.json` files that describe the cloud providers and VMs to run the benchmarks on. Is unmarshalled into `[]CloudDetails`.
- `deployment-steps.md`: Instructions for provisioning VMs, or a high-level overview for what we expect to do via `roachprod`.
- `googleSheets.go`: Contains all of the code to actually post results to Google Sheets.
- `init.sh`: An initialization script placed on machines to prep them for the benchmarks. Because it contains instructions to unzip the `scripts` dir, it should be left outside the `scripts` dir.
- `logs`: Initialization and run logs of the VMs. Automatically populated by the binary.
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

**Note:** For the binary to work, you should also have a `credentials.json` and `token.json` file in the root directory of this project. Both of which are part of setting up Google Sheets access.
