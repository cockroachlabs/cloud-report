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
	"os/user"
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
var crlUsername = flag.String("u", "", "CRL username, if different from `whoami`")

// TODO: leaving benchmark list as a record of what needs to be migrated.
// benchmark describes the benchmark you want to run and its outputs.
// type benchmark struct {
// 	// Name to print when benchmark is running.
// 	name string
// 	// benchmarkRoutine to run.
// 	routines []benchmarkRoutine
// 	// artifacts to download at end of run.
// 	artifacts []artifact
//
// 	// disable this benchmark
// 	disabled bool
// }
//
// var benchmarks = []benchmark{
// 	{
// 		disabled: true,
// 		name:     "ping",
// 		routines: []benchmarkRoutine{{
// 			file: "./scripts/gen/network-ping.sh",
// 			arg:  argNode2InternalIP,
// 			node: 1,
// 		}},
// 		artifacts: []artifact{{"~/network-ping.log", 1}},
// 	},
// 	{
// 		name: "netperf",
// 		routines: []benchmarkRoutine{{
// 			file: "./scripts/gen/network-netperf.sh",
// 			arg:  argNode2InternalIP,
// 			node: 1,
// 		}},
// 		artifacts: []artifact{{"~/netperf-results.log", 1}},
// 	},
// 	{
// 		name: "cpu",
// 		routines: []benchmarkRoutine{{
// 			file: "./scripts/gen/cpu.sh",
// 			node: 1,
// 		}},
// 		artifacts: []artifact{{"~/cpu.log", 1}},
// 	},
// 	{
// 		name: "iperf",
// 		routines: []benchmarkRoutine{
// 			{
// 				name:              "client",
// 				file:              "./scripts/gen/network-iperf-client.sh",
// 				arg:               argNode2InternalIP,
// 				launchAsGoroutine: true,
// 				node:              1,
// 			},
// 			{
// 				name:              "client",
// 				file:              "./scripts/gen/network-iperf-client.sh",
// 				arg:               argNode2InternalIP,
// 				launchAsGoroutine: true,
// 				node:              3,
// 			},
// 			{
// 				name:              "client",
// 				file:              "./scripts/gen/network-iperf-client.sh",
// 				arg:               argNode2InternalIP,
// 				launchAsGoroutine: true,
// 				node:              4,
// 			},
// 			{
// 				name: "server",
// 				file: "./scripts/gen/network-iperf-server.sh",
// 				node: 2,
// 			},
// 		},
// 		artifacts: []artifact{
// 			{"~/network-iperf-server.log", 2},
// 		},
// 	},
// 	{
// 		disabled: true,
//
// 		name: "io",
// 		routines: []benchmarkRoutine{
// 			{
// 				name: "load",
// 				file: "./scripts/gen/io-load.sh",
// 				arg:  argCloudName,
// 				node: 1,
// 			},
// 			{
// 				name: "write",
// 				file: "./scripts/gen/io-wr.sh",
// 				node: 1,
// 			},
// 			{
// 				name: "read",
// 				file: "./scripts/gen/io-rd.sh",
// 				node: 1,
// 			},
// 		},
// 		artifacts: []artifact{
// 			{"/mnt/data1/io-load-results.log", 1},
// 			{"/mnt/data1/io-wr-results.log", 1},
// 			{"/mnt/data1/io-rd-results.log", 1},
// 		},
// 	},
// 	{
// 		name: "fio",
// 		routines: []benchmarkRoutine{
// 			{
// 				name: "write",
// 				file: "./scripts/gen/fio.sh",
// 				node: 1,
// 				arg:  "/mnt/data1",
// 			},
// 		},
// 		artifacts: []artifact{
// 			{"/mnt/data1/fio/fio-report.log", 1},
// 		},
// 	},
// }

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
set -ex
CLOUD="{{.CloudDetails.Cloud}}"
CLUSTER="{{.Cluster}}"
NODES=4

# Create roachprod cluster
function create() {
  type="$CLOUD"
  if [ "$CLOUD" = "gcp" ]
  then
    type="gce"
  fi

  roachprod create "$CLUSTER" -n $NODES --lifetime "{{.Lifetime}}" --clouds "$CLOUD" \
    --${type}-machine-type "{{.MachineType}}" {{.EvaledArgs}}
}

# Setup and configure roachprod cluster.
function setup() {
	{{.ScriptsDir}}/setup.sh "$CLOUD" "$CLUSTER"
}

# Run CPU benchmark
function bench_cpu() {
  roachprod run "$CLUSTER":1  ./scripts/gen/cpu.sh
  #TODO: download artifacts
}

# Commands to execute specified on a command line
# TODO: we assume the order of commands makes sense (i.e. create before setup).
cmds=("$@")
if [ ${#cmds[@]} -eq 0 ]; then
  # If not specified, run all commands
  cmds=("create" "setup" "cpu" "io" "net" "tpcc")
fi

for cmd in "${cmds[@]}"
do
  case $cmd in
  create)
    create $*
  ;;
  setup)
    setup $*
  ;;
  cpu)
    bench_cpu $*
  ;;
  io)
    implement_me $*
  ;;
  net)
    implement_me $*
  ;;
  tpcc)
    implement_me $*
  ;;
  *)
    echo "Usage: $0 [ create | setup | cpu | io | net | tpcc ]" >&2
  ;;
  esac
done
`

func evalArgs(args map[string]string, templateArgs scriptData, buf *bytes.Buffer) error {
	for arg, val := range args {
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(buf, "--%s", arg)
		if len(val) > 0 {
			evaledArg := template.Must(template.New("arg").Parse(fmt.Sprintf(" %q", val)))
			if err := evaledArg.Execute(buf, templateArgs); err != nil {
				return fmt.Errorf("error evaluating arg %s: %v", arg, err)
			}
		}
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

	clusterPrefix := *crlUsername + "-cldrprt21-micro"

	scriptTemplate := template.Must(template.New("script").Parse(driverTemplate))

	for machineType, machineArgs := range cloud.MachineTypes {
		clusterName := fmt.Sprintf("%s-%s-%s-%s-%s", clusterPrefix, cloud.Cloud, cloud.Group, *reportVersion, machineType)
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
		evaledArgs := bytes.NewBuffer(nil)
		if err := evalArgs(cloud.RoachprodArgs, templateArgs, evaledArgs); err != nil {
			return err
		}
		if err := evalArgs(machineArgs, templateArgs, evaledArgs); err != nil {
			return err
		}
		templateArgs.EvaledArgs = evaledArgs.String()

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

	if *crlUsername == "" {
		u, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		*crlUsername = u.Username
	}

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
