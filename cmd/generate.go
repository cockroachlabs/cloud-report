// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.
package cmd

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

var scriptsDir string
var lifetime string

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates scripts necessary for execution of cloud report benchmarks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, cloud := range clouds {
			if err := generateCloudScripts(cloud); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&scriptsDir, "scripts-dir", "",
		"./scripts", "directory containing scripts uploaded to cloud VMs that execute benchmarks.")
	generateCmd.Flags().StringVarP(&lifetime, "lifetime", "l",
		"4h", "cluster lifetime")
}

type scriptData struct {
	CloudDetails
	Cluster          string
	Lifetime         string
	MachineType      string
	ScriptsDir       string
	EvaledArgs       string
	UsEastAmi        string
	UsWestAmi        string
	NodeEastLocation string
	NodeWestLocation string
	BenchArgs        map[string]string
}

const driverTemplate = `#!/bin/bash

CLOUD="{{.CloudDetails.Cloud}}"
CLUSTER="$CRL_USERNAME-{{.Cluster}}"
NODES=4
TMUX_SESSION="cloud-report"
WEST_CLUSTER="${CLUSTER}-west"
WEST_CLUSTER_CREATED=

set -ex
scriptName=$(basename ${0%.*})
logdir="$(dirname $0)/../logs/${scriptName}"
mkdir -p "$logdir"

# Redirect stdout and stderr into script log file
exec &> >(tee -a "$logdir/driver.log")

# Create roachprod cluster
function create_cluster() {
  roachprod create "$CLUSTER" -n $NODES --lifetime "{{.Lifetime}}" --clouds "$CLOUD" \
    --$CLOUD-machine-type "{{.MachineType}}" {{.NodeEastLocation}} {{.EvaledArgs}} {{.UsEastAmi}}
  roachprod run "$CLUSTER" -- tmux new -s "$TMUX_SESSION" -d
}

# Create roachprod in us-west2
function create_west_cluster() {
  roachprod create "$WEST_CLUSTER" -u $USER -n $NODES --lifetime "4h" --clouds "$CLOUD" \
    --$CLOUD-machine-type "{{.MachineType}}" {{.NodeWestLocation}} {{.EvaledArgs}} {{.UsWestAmi}}
  roachprod run "$WEST_CLUSTER" -- tmux new -s "$TMUX_SESSION" -d
  WEST_CLUSTER_CREATED=true
}

# Upload scripts to roachprod cluster
function upload_scripts() {
  roachprod run "$1" rm  -- -rf ./scripts
  roachprod put "$1" {{.ScriptsDir}} scripts
  roachprod run "$1" chmod -- -R +x ./scripts
}

# Load the cockroach binary to roachprod cluster
function load_cockroach() {
  roachprod run "$1" "rm -f ./cockroach"
  if [ -z "$cockroach_binary" ]
  then
    roachprod stage "$1" cockroach
  else
    roachprod put "$1" "$cockroach_binary" "cockroach"
  fi
}

# Start cockroach cluster on nodes [1-NODES-1].
function start_cockroach() {
  # Build --store flags based on the number of disks.
  # Roachprod adds /mnt/data1/cockroach by itself, so, we'll pick up the other disks
  for s in $(roachprod run "$CLUSTER":1 'ls -1d /mnt/data[2-9]* 2>/dev/null || echo')
  do
   stores="$stores --store $s/cockroach"
  done

  if [[ -z $stores ]]; then
    stores="--store=/mnt/data1/cockroach"
  fi

  if [[ $NODES == 2 ]]; then
  	roachprod start "$CLUSTER":1 --args="$stores --cache=0.25 --max-sql-memory=0.4"
  else
  	roachprod start "$CLUSTER":1-$((NODES-1)) --args="$stores --cache=0.25 --max-sql-memory=0.4"
  fi
}

# Execute setup.sh script on the cluster to configure it
function setup_cluster() {
	roachprod run "$1" sudo ./scripts/gen/setup.sh "$CLOUD"
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
  run_under_tmux "io" "$CLUSTER:1" "./scripts/gen/fio.sh $io_extra_args"
}

# Wait for FIO benchmark top finish and retrieve results.
function fetch_bench_io_results() {
  roachprod run "$CLUSTER":1 ./scripts/gen/fio.sh -- -w
  roachprod get "$CLUSTER":1 ./fio-results $(results_dir "fio-results")
}

# Run Netperf benchmark
function bench_net() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  server=$(roachprod ip "$CLUSTER":$NODES)
  port=1337
  # Start server
  roachprod run "$CLUSTER":$NODES ./scripts/gen/network-netperf.sh -- -S -p $port

  # Start client
  run_under_tmux "net" "$CLUSTER:$((NODES-1))" "./scripts/gen/network-netperf.sh -s $server -p $port $net_extra_args"
}

# Wait for Netperf benchmark to complete and fetch results.
function fetch_bench_net_results() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  if [ $NODES -eq 2 ]
  then
    roachprod run "$CLUSTER":1 ./scripts/gen/network-netperf.sh -- -w
    roachprod get "$CLUSTER":1 ./netperf-results $(results_dir "netperf-results")
  else
    roachprod run "$CLUSTER":3 ./scripts/gen/network-netperf.sh -- -w
    roachprod get "$CLUSTER":3 ./netperf-results $(results_dir "netperf-results")
  fi
}

# Run TPCC Benchmark
function bench_tpcc() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  fi

  start_cockroach
  if [ $NODES -eq 2 ]
  then
    pgurls=$(roachprod pgurl "$CLUSTER":1)
    run_under_tmux "tpcc" "$CLUSTER:2" "./scripts/gen/tpcc.sh $tpcc_extra_args ${pgurls[@]}"
  else
    pgurls=$(roachprod pgurl "$CLUSTER":1-$((NODES-1)))
    run_under_tmux "tpcc" "$CLUSTER:$NODES" "./scripts/gen/tpcc.sh $tpcc_extra_args ${pgurls[@]}"
  fi
}

function fetch_bench_tpcc_results() {
  if [ $NODES -lt 2 ]
  then
    echo "NODES must be greater than 1 for this test"
    exit 1
  else
    roachprod run "$CLUSTER":$NODES ./scripts/gen/tpcc.sh -- -w
    roachprod get "$CLUSTER":$NODES ./tpcc-results $(results_dir "tpcc-results")	
  fi
}

function bench_cross_region_net() {
  create_west_cluster
  upload_scripts "$WEST_CLUSTER"
  roachprod run ${WEST_CLUSTER}:1 "sudo apt-get update && sudo apt-get -y install netperf"

  server_ip=$(roachprod ip ${CLUSTER}:1)
  
  port=1337
  # Server
  roachprod run ${CLUSTER}:1 ./scripts/gen/network-netperf.sh -- -S -p "$port"
  # Cient
  run_under_tmux "cross_region_net" ${WEST_CLUSTER}:1 "./scripts/gen/network-netperf.sh -s $server_ip -p $port $cross_region_net_extra_args"
}

function fetch_bench_cross_region_net_results() {
  roachprod run ${WEST_CLUSTER}:1 ./scripts/gen/network-netperf.sh -- -w
  roachprod get ${WEST_CLUSTER}:1 ./netperf-results $(results_dir "netperf-results")
}

# Destroy roachprod cluster
function destroy_cluster() {
  roachprod destroy "$CLUSTER"
  if [[ -n $WEST_CLUSTER_CREATED ]]; then
    roachprod destroy "$WEST_CLUSTER"
  fi
}

function usage() {
echo "$1
Usage: $0 [-b <bootstrap>]... [-w <workload>]... [-d] [-c cockroach_binary]
   -b: One or more bootstrap steps.
         -b create: creates cluster
         -b upload: uploads required scripts
         -b setup: execute setup script on the cluster
         -b all: all of the above steps
   -w: Specify workloads (benchmarks) to execute.
       -w cpu : Benchmark CPU
       -w io  : Benchmark IO
       -w net : Benchmark Net
       -w cr_net : Benchmark Cross-region Net
       -w tpcc: Benchmark TPCC
       -w all : All of the above
   -c: Override cockroach binary to use.
   -r: Do not start benchmarks specified by -w.  Instead, resume waiting for their completion.
   -I: additional IO benchmark arguments
   -N: additional network benchmark arguments
   -C: additional CPU benchmark arguments
   -T: additional TPCC benchmark arguments
   -R: additional cross-region network benchmark arguments
   -n: override number of nodes in a cluster
   -d: Destroy cluster
"
exit 1
}

benchmarks=()
f_resume=''
do_create=''
do_upload=''
do_setup=''
do_destroy=''
io_extra_args='{{with $arg := .BenchArgs.io}}{{$arg}}{{end}}'
cpu_extra_args='{{with $arg := .BenchArgs.cpu}}{{$arg}}{{end}}'
net_extra_args='{{with $arg := .BenchArgs.net}}{{$arg}}{{end}}'
tpcc_extra_args='{{with $arg := .BenchArgs.tpcc}}{{$arg}}{{end}}'
cross_region_net_extra_args='{{with $arg := .BenchArgs.cross_region_net}}{{$arg}}{{end}}'
cockroach_binary=''

while getopts 'c:b:w:dn:I:N:C:T:R:r' flag; do
  case "${flag}" in
    b) case "${OPTARG}" in
        all)
          do_create='true'
          do_upload='true'
          do_setup='true'
          do_cockroach='true'
        ;;
        create)    do_create='true' ;;
        upload)    do_upload='true' ;;
        setup)     do_setup='true' ;;
        *) usage "Invalid -b value '${OPTARG}'" ;;
       esac
    ;;
    c) cockroach_binary="${OPTARG}" ;;
    w) case "${OPTARG}" in
         cpu) benchmarks+=("bench_cpu") ;;
         io) benchmarks+=("bench_io") ;;
         net) benchmarks+=("bench_net") ;;
         cr_net) benchmarks+=("bench_cross_region_net") ;;
         tpcc) benchmarks+=("bench_tpcc") ;;
         all) benchmarks+=("bench_cpu" "bench_io" "bench_net" "bench_tpcc" "bench_cross_region_net") ;;
         *) usage "Invalid -w value '${OPTARG}'";;
       esac
    ;;
    d) do_destroy='true' ;;
    r) f_resume='true' ;;
    n) NODES="${OPTARG}" ;;
    I) io_extra_args="${OPTARG}" ;;
    C) cpu_extra_args="${OPTARG}" ;;
    N) net_extra_args="${OPTARG}" ;;
    T) tpcc_extra_args="${OPTARG}" ;;
    R) cross_region_net_extra_args="${OPTARG}" ;;
    *) usage ;;
  esac
done

if [ -n "$do_create" ];
then
  create_cluster
fi

if [ -n "$do_upload" ];
then
  upload_scripts $CLUSTER
  load_cockroach $CLUSTER
fi

if [ -n "$do_setup" ];
then
  setup_cluster $CLUSTER
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

if [ -n "$do_destroy" ];
then 
  destroy_cluster
fi
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

func FormatMachineType(m string) string {
	return strings.Replace(m, ".", "-", -1)
}

func hashStrings(vals ...string) uint32 {
	hasher := crc32.NewIEEE()
	for _, v := range vals {
		_, _ = hasher.Write([]byte(v))
	}
	return hasher.Sum32()
}

func generateCloudScripts(cloud CloudDetails) error {
	if err := makeAllDirs(cloud.BasePath(), cloud.ScriptDir(), cloud.LogDir()); err != nil {
		return err
	}

	scriptTemplate := template.Must(template.New("script").Parse(driverTemplate))
	for machineType, machineConfig := range cloud.MachineTypes {
		clusterName := fmt.Sprintf("cldrprt%d-%s-%d",
			(1+time.Now().Year())%1000, machineType,
			hashStrings(cloud.Cloud, cloud.Group, reportVersion))
		validClusterName := regexp.MustCompile(`[\.|\_]`)
		clusterName = validClusterName.ReplaceAllString(clusterName, "-")

		templateArgs := scriptData{
			CloudDetails: cloud,
			Cluster:      clusterName,
			Lifetime:     lifetime,
			MachineType:  machineType,
			ScriptsDir:   scriptsDir,
			BenchArgs:    combineArgs(machineConfig.BenchArgs, cloud.BenchArgs),
		}

		// Evaluate roachprodArgs: those maybe templatized.
		evaledArgs := make(map[string]string)
		combinedArgs := combineArgs(machineConfig.RoachprodArgs, cloud.RoachprodArgs)
		if err := evalArgs(combinedArgs, templateArgs, evaledArgs); err != nil {
			return err
		}

		buf := bytes.NewBuffer(nil)
		for arg, val := range evaledArgs {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
			switch arg {
			case "gce-zones", "aws-zones", "azure-locations":
				if !(len(val) > 0) {
					return fmt.Errorf("zone config for %s is no specified", arg)
				}
				templateArgs.NodeEastLocation = fmt.Sprintf("--%s=%q", arg, val)
			case "west-gce-zones", "west-aws-zones", "west-azure-locations":
				if !(len(val) > 0) {
					return fmt.Errorf("zone config for %s is no specified", arg)
				}
				templateArgs.NodeWestLocation = fmt.Sprintf("--%s=%q", arg[5:], val)
			case "aws-image-ami", "gce-image":
				templateArgs.UsEastAmi = fmt.Sprintf("--%s=%q", arg, val)
			case "west-aws-image-ami", "west-gce-image":
				templateArgs.UsWestAmi = fmt.Sprintf("--%s=%q", arg[5:], val)
			default:
				fmt.Fprintf(buf, "--%s", arg)
				if len(val) > 0 {
					fmt.Fprintf(buf, "=%q", val)
				}
			}
		}
		templateArgs.EvaledArgs = buf.String()

		scriptName := path.Join(
			cloud.ScriptDir(),
			fmt.Sprintf("%s.sh", FormatMachineType(machineType)))
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