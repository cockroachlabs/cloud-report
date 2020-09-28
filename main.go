package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"
)

var lifetime = flag.String("lifetime", "12h", "cluster lifetime")
var cloudDetailsFile = flag.String("cloud-details", "./cloudDetails/default.json", "run tests against specified input, which will be loaded into clouds")
var baseOutputDir = flag.String("output-dir", "./report-data", "directory to emit results and scripts")
var scriptsDir = flag.String("scripts-dir", "./scripts", "directory containing cloud benchmark scripts")
var reportVersion = flag.String("report-version", time.Now().Format("20060102"), "subdirectory to write data to")

// CloudDetails provides the name of the cloud and the different
// machine types you should run the benchmark suite against.
type CloudDetails struct {
	Cloud string `json:"cloud"`
	Group string `json:"group"`

	// Common arguments passed to roachprod create.
	RoachprodArgs map[string]string `json:"roachprodArgs"`

	// Map from machine type to the map of the machine specific arguments
	// that should be passed when creating cluster.
	MachineTypes map[string]map[string]string `json:"machineTypes"`
}

func makeAllDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

type scriptData struct {
	CloudDetails
	Cluster     string
	Lifetime    string
	MachineType string
	ScriptsDir  string
	EvaledArgs  string
}

const driverTemplate = `#!/bin/bash

CLOUD="{{.CloudDetails.Cloud}}"
CLUSTER="$USER-{{.Cluster}}"
NODES=4
TMUX_SESSION="cloud-report"

set -ex
scriptName=$(basename ${0%.*})
logdir="$(dirname $0)/../logs/${scriptName}"
mkdir -p "$logdir"

# Redirect stdout and stderr into script log file
exec &> >(tee -a "$logdir/driver.log")

# Create roachprod cluster
function create_cluster() {
  roachprod create "$CLUSTER" -n $NODES --lifetime "{{.Lifetime}}" --clouds "$CLOUD" \
    --$CLOUD-machine-type "{{.MachineType}}" {{.EvaledArgs}}
  roachprod run "$CLUSTER" -- tmux new -s "$TMUX_SESSION" -d
}

# Upload scripts to roachprod cluster
function upload_scripts() {
  roachprod run "$CLUSTER" rm  -- -rf ./scripts
  roachprod put "$CLUSTER" {{.ScriptsDir}} scripts
  roachprod run "$CLUSTER" chmod -- -R +x ./scripts
}

# Execute setup.sh script on the cluster to configure it
function setup_cluster() {
	roachprod run "$CLUSTER" sudo ./scripts/gen/setup.sh "$CLOUD"
}

# executes command on a host using roachprod, under tmux session.
function run_under_tmux() {
  local name=$1
  local host=$2
  local cmd=$3
  roachprod run $host -- tmux neww -t "$TMUX_SESSION" -n "$name" -d -- "$cmd"
}

#
# Benchmark scripts should execute a single benchmark
# and download results to the $logdir directory.
# results_dir returns date suffixed directory under logdir.
#
function results_dir() {
  echo "$logdir/$1.$(date +%Y%m%d.%T)"
}

# Run CPU benchmark
function bench_cpu() {
  run_under_tmux "cpu" "$CLUSTER:1"  "./scripts/gen/cpu.sh $cpu_extra_args"
}

# Wait for CPU benchmark to finish and retrieve results.
function fetch_bench_cpu_results() {
  roachprod run "$CLUSTER":1  ./scripts/gen/cpu.sh -- -w
  roachprod get "$CLUSTER":1 ./coremark-results $(results_dir "coremark-results")
}

# Run FIO benchmark
function bench_io() {
  run_under_tmux "io" "$CLUSTER:2" "./scripts/gen/fio.sh -- $io_extra_args"
}

# Wait for FIO benchmark top finish and retrieve results.
function fetch_bench_io_results() {
  roachprod run "$CLUSTER":2 ./scripts/gen/fio.sh -- -w
  roachprod get "$CLUSTER":2 ./fio-results $(results_dir "fio-results")
}

# Run Netperf benchmark
function bench_net() {
  server=$(roachprod ip "$CLUSTER":4)
  port=1337
  # Start server
  roachprod run "$CLUSTER":4 ./scripts/gen/network-netperf.sh -- -S -p $port

  # Start client
  run_under_tmux "net" "$CLUSTER:3" "./scripts/gen/network-netperf.sh -s $server -p $port $net_extra_args"
}

# Wait for Netperf benchmark to complete and fetch results.
function fetch_bench_net_results() {
  roachprod run "$CLUSTER":3 ./scripts/gen/network-netperf.sh -- -w
  roachprod get "$CLUSTER":3 ./netperf-results $(results_dir "netperf-results")	
}

# Run TPCC Benchmark
function bench_tpcc() {
  echo "IMPLEMENT ME" $tpcc_extra_args
}

# Destroy roachprod cluster
function destroy_cluster() {
  roachprod destroy "$CLUSTER"
}

function usage() {
echo "$1
Usage: $0 [-b <bootstrap>]... [-w <workload>]... [-d]
   -b: One or more bootstrap steps.
         -b create: creates cluster
         -b upload: uploads required scripts
         -b setup: execute setup script on the cluster
         -b all: all of the above steps
   -w: Specify workloads (benchmarks) to execute.
       -w cpu : Benchmark CPU
       -w io  : Benchmark IO
       -w net : Benchmark Net
       -w tpcc: Benchmark TPCC
       -w all : All of the above
   -r: Do not start benchmarks specified by -w.  Instead, resume waiting for their completion.
   -I: additional IO benchmark arguments
   -N: additional network benchmark arguments
   -C: additional CPU benchmark arguments
   -T: additional TPCC benchmark arguments
   -d: Destroy cluster
"
exit 1
}

benchmarks=()
f_resume=''
do_create=''
do_upload=''
do_setup=''
io_extra_args=''
cpu_extra_args=''
net_extra_args=''
tpcc_extra_args=''

while getopts 'b:w:dI:N:C:T:r' flag; do
  case "${flag}" in
    b) case "${OPTARG}" in
        all)
          do_create='true'
          do_upload='true'
          do_setup='true'
        ;;
        create) do_create='true' ;;
        upload) do_upload='true' ;;
        setup)  do_setup='true' ;;
        *) usage "Invalid -b value '${OPTARG}'" ;;
       esac
    ;;
    w) case "${OPTARG}" in
         cpu) benchmarks+=("bench_cpu") ;;
         io) benchmarks+=("bench_io") ;;
         net) benchmarks+=("bench_net") ;;
         tpcc) benchmarks+=("bench_tpcc") ;;
         all) benchmarks+=("bench_cpu" "bench_io" "bench_net" "bench_tpcc") ;;
         *) usage "Invalid -w value '${OPTARG}'";;
       esac
    ;;
    d) destroy_cluster
       exit 0
    ;;
    r) f_resume='true' ;;
    I) io_extra_args="${OPTARG}" ;;
    C) cpu_extra_args="${OPTARG}" ;;
    N) net_extra_args="${OPTARG}" ;;
    T) tpcc_extra_args="${OPTARG}" ;;
    *) usage ;;
  esac
done

if [ -n "$do_create" ];
then
  create_cluster
fi

if [ -n "$do_upload" ];
then
  upload_scripts
fi

if [ -n "$do_setup" ];
then
  setup_cluster
fi

if [ -z "$f_resume" ]
then
  # Execute requested benchmarks.
  for bench in "${benchmarks[@]}"
  do
    $bench
  done
fi

# Wait for benchmarks to finsh and fetch their results.
for bench in "${benchmarks[@]}"
do
  echo "Waiting for $bench to complete"
  fetch="fetch_${bench}_results"
  $fetch
done
`

// combineArgs takes base arguments applicable to the cloud and machine specific
// args and combines them by specializing machine specific args if there is a
// conflict.
func combineArgs(machineArgs map[string]string, baseArgs map[string]string) map[string]string {
	if machineArgs == nil {
		return baseArgs
	}
	for arg, val := range baseArgs {
		if _, found := machineArgs[arg]; !found {
			machineArgs[arg] = val
		}
	}
	return machineArgs
}

func evalArgs(
	inputArgs map[string]string, templateArgs scriptData, evaledArgs map[string]string,
) error {
	for arg, val := range inputArgs {
		buf := bytes.NewBuffer(nil)
		if err := template.Must(template.New("arg").Parse(val)).Execute(buf, templateArgs); err != nil {
			return fmt.Errorf("error evaluating arg %s: %v", arg, err)
		}
		evaledArgs[arg] = buf.String()
	}
	return nil
}

func generateScripts(cloud CloudDetails) error {
	basePath := path.Join(*baseOutputDir, cloud.Cloud, cloud.Group)
	scriptDir := path.Join(basePath, "scripts")
	logDir := path.Join(basePath, "logs")

	if err := makeAllDirs(basePath, scriptDir, logDir); err != nil {
		return err
	}

	scriptTemplate := template.Must(template.New("script").Parse(driverTemplate))
	for machineType, machineArgs := range cloud.MachineTypes {
		clusterName := fmt.Sprintf("cldrprt%d-%s-%s-%s-%s",
			(1+time.Now().Year())%1000, cloud.Cloud, cloud.Group, *reportVersion, machineType)
		validClusterName := regexp.MustCompile(`[\.|\_]`)
		clusterName = validClusterName.ReplaceAllString(clusterName, "-")

		templateArgs := scriptData{
			CloudDetails: cloud,
			Cluster:      clusterName,
			Lifetime:     *lifetime,
			MachineType:  machineType,
			ScriptsDir:   *scriptsDir,
		}

		// Evaluate roachprodArgs: those maybe templatized.
		evaledArgs := make(map[string]string)
		combinedArgs := combineArgs(machineArgs, cloud.RoachprodArgs)
		if err := evalArgs(combinedArgs, templateArgs, evaledArgs); err != nil {
			return err
		}

		buf := bytes.NewBuffer(nil)
		for arg, val := range evaledArgs {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
			fmt.Fprintf(buf, "--%s", arg)
			if len(val) > 0 {
				fmt.Fprintf(buf, " %q", val)
			}
		}
		templateArgs.EvaledArgs = buf.String()

		scriptName := path.Join(
			scriptDir,
			fmt.Sprintf("%s.sh", strings.Replace(machineType, ".", "-", -1)))
		f, err := os.OpenFile(scriptName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}

		if err := scriptTemplate.Execute(f, templateArgs); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	// Check roachprod.
	_, err := exec.LookPath("roachprod")
	if err != nil {
		log.Fatal("Install roachprod in your $PATH")
	}

	// Parse cloud configuration file
	f, err := os.Open(*cloudDetailsFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	b, _ := ioutil.ReadAll(f)

	var clouds []CloudDetails
	err = json.Unmarshal(b, &clouds)
	if err != nil {
		log.Fatal(err)
	}

	// Setup output directory.
	*baseOutputDir = path.Join(*baseOutputDir, *reportVersion)
	log.Printf("Generating scripts under %s", *baseOutputDir)
	if err := os.MkdirAll(*baseOutputDir, 0755); err != nil {
		log.Fatal(err)
	}

	// Generate scripts.
	for _, cloud := range clouds {
		if err := generateScripts(cloud); err != nil {
			log.Fatal(err)
		}
	}
	// TODO: add more commands (generate, analyze, etc).
}
