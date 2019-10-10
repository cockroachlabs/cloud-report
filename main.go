package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ioSkip = flag.Bool("io-skip", false, "skip the IO tests, which take a long time to run.")
var ioOnly = flag.Bool("io-only", false, "only run the IO tests.")
var loadOnly = flag.Bool("load-only", false, "load the scripts but do not run the benchmarks.")
var iterations = flag.Int("iterations", 1, "run the benchmarks on the same machines {iterations} number of times.")
var cloudDetailsFile = flag.String("cloudDetails", "./cloudDetails/default.json", "run tests against specified input, which will be loaded into clouds")
var crlUsername = flag.String("u", "", "CRL username, if different from `whoami`")

var runAzure = flag.Bool("azure", false, "run microbenchmarks on Azure VMs you've already provisioned.")
var runOnPrem = flag.Bool("on-prem", false, "run microbenchmarks on arbitrary VMs you've already provisioned.")
var machineName = flag.String("machine-name", "", "The name of the machine; used to track results for on-prem runs")

var node1IP = flag.String("node1", "", "IP address of node 1")
var node2IP = flag.String("node2", "", "IP address of node 2")
var node2InternalIP = flag.String("node2-internal", "", "The internal IP address of node 2; used by node 1 in networking tests")

// These consts represent all of the arguments that
const (
	argNode2InternalIP = "node2IP"
	argCloudName       = "cloudName"
)

var argConstArr = []string{argNode2InternalIP, argCloudName}

// checkForAllArgs checks that all of the expected values are present
// in the argVals map passed to platformRunner.run().
func checkForAllArgs(f *os.File, argVals map[string]string) {
	for _, arg := range argConstArr {
		_, ok := argVals[arg]
		if !ok {
			fmt.Fprintf(f, "argVals missing %s in an execution of platformRunner.run()", arg)
			os.Exit(1)
		}
	}
}

// benchmarkRoutine lets you execute multiple scripts as one benchmark.
type benchmarkRoutine struct {
	// Optional name to print when routine is running.
	name string
	// Script containing routine.
	file string
	// Name of argument to pass. Should be defined as constant above,
	// beginning with arg...
	arg string
	// Launch routine as goroutine. Note that there is nothing to synchronize
	// the goroutine's execution currently, so you cannot reliably download
	// artifacts from processes started as go routines.
	launchAsGoroutine bool
	// Which node should run this (counting starts at 1).
	node int
}

// artifacts are files that you expect to be generated on nodes after the
// benchmark completes. If the specified artifact DNE on the node, the
// program terminates.
type artifact struct {
	file string
	node int
}

// benchmark describes the benchmark you want to run and its outputs.
type benchmark struct {
	// Name to print when benchmark is running.
	name string
	// benchmarkRoutine to run.
	routines []benchmarkRoutine
	// artifacts to download at end of run.
	artifacts []artifact
}

var benchmarks = []benchmark{
	{
		name: "ping",
		routines: []benchmarkRoutine{{
			file: "./scripts/gen/network-ping.sh",
			arg:  argNode2InternalIP,
			node: 1,
		}},
		artifacts: []artifact{{"~/network-ping.log", 1}},
	},
	{
		name: "cpu",
		routines: []benchmarkRoutine{{
			file: "./scripts/gen/cpu.sh",
			node: 1,
		}},
		artifacts: []artifact{{"~/cpu.log", 1}},
	},
	{
		name: "iperf",
		routines: []benchmarkRoutine{
			{
				name:              "server",
				file:              "./scripts/gen/network-iperf-server.sh",
				launchAsGoroutine: true,
				node:              2,
			},
			{
				name: "client",
				file: "./scripts/gen/network-iperf-client.sh",
				arg:  argNode2InternalIP,
				node: 1,
			},
		},
		artifacts: []artifact{
			{"~/network-iperf-client.log", 1},
		},
	},
	{
		name: "io",
		routines: []benchmarkRoutine{
			{
				name: "load",
				file: "./scripts/gen/io-load.sh",
				arg:  argCloudName,
				node: 1,
			},
			{
				name: "write",
				file: "./scripts/gen/io-wr.sh",
				node: 1,
			},
			{
				name: "read",
				file: "./scripts/gen/io-rd.sh",
				node: 1,
			},
		},
		artifacts: []artifact{
			{"/mnt/data1/io-load-results.log", 1},
			{"/mnt/data1/io-wr-results.log", 1},
			{"/mnt/data1/io-rd-results.log", 1},
		},
	},
}

// CloudDetails provides the name of the cloud and the different
// machine types you should run the benchmark suite against.
type CloudDetails struct {
	Name            string   `json:"name"`
	MachineTypes    []string `json:"machineTypes"`
	EBSMachineTypes []string `json:"ebsMachineTypes"`
}

// runCmd is a convenience function around exec.Command.
func runCmd(f *os.File, name string, arg ...string) (stdoutStr string) {
	// 45 minutes is the longest any test runs (io) plus a few extra
	// minutes as a buffer.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, arg...)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		fmt.Fprint(f, "Command timed out")
		runtime.Goexit()
	}

	if err != nil {
		fmt.Fprint(f, "Died on command", cmd)
		fmt.Fprint(f, stdout.String())
		fmt.Fprint(f, stderr.String())
		fmt.Fprint(f, err)
		runtime.Goexit()
	}
	return stdout.String()
}

// runCmdReturnString attempts to convert it to a string without
// escape characters or trailing line breaks.
func runCmdReturnString(f *os.File, name string, arg ...string) string {
	strVal := runCmd(f, name, arg...)
	strVal = strings.Replace(strVal, "\"", "", -1)
	return strings.TrimRight(strVal, "\n")
}

// runCmdFindString searches a command's output for the given string
// and confirms its presence or absence.
func runCmdFindString(f *os.File, searchString, name string, arg ...string) bool {
	strVal := runCmd(f, name, arg...)

	stringFound := strings.Contains(strVal, searchString)

	return stringFound
}

// createDir is a convenience function to create a directory using the
// naming schema that we've determined for the cloud report.
func createDir(f *os.File, cloudName, machineType string) string {
	dateString := time.Now().Format("20060102")
	resultsParentPath := fmt.Sprintf("results/%s/%s/%s", cloudName, machineType, dateString)
	if _, err := os.Stat(resultsParentPath); os.IsNotExist(err) {
		os.MkdirAll(resultsParentPath, os.ModePerm)
	}
	files, err := ioutil.ReadDir(resultsParentPath)
	if err != nil {
		fmt.Fprint(f, err)
		log.Fatal(err)
	}
	resultsPath := fmt.Sprintf("%s/%s", resultsParentPath, strconv.Itoa(len(files)))
	os.Mkdir(resultsPath, os.ModePerm)

	return resultsPath
}

func isIPWellFormed(ipString string) bool {
	ip := net.ParseIP(ipString)

	return ip != nil
}

// platformRunner lets you describe methods that allow arbitrary platforms
// upload, run, and download files.
type platformRunner struct {
	upload   func(f *os.File, dest, file string)
	exec     func(f *os.File, src, cmd string)
	download func(f *os.File, src, file, path string)
}

// roachprodRunner relies on the roachprod binary to manage remote
// clusters, including creation.
var roachprodRunner = platformRunner{
	upload: func(f *os.File, dest, file string) {
		runCmd(f, "roachprod", "put", dest, file)
	},
	exec: func(f *os.File, src, cmd string) {
		runCmd(f, "roachprod", "run", src, cmd)
	},
	download: func(f *os.File, src, file, path string) {
		runCmd(f, "roachprod", "get", src, file, path)
	},
}

// shellRunner relies on shell features to execute operations on remote
// machines. Notably, it expects the remote machines to have its SSH key
// accessible at ~/.ssh/id_rsa.
var shellRunner = platformRunner{
	upload: func(f *os.File, dest, file string) {
		rootOfDest := dest + ":~"
		runCmd(f, "scp", file, rootOfDest)
	},
	exec: func(f *os.File, src, cmd string) {
		runCmd(f, "ssh", src, cmd)
	},
	download: func(f *os.File, src, file, path string) {
		remoteFile := fmt.Sprintf("%s:%s", src, file)
		runCmd(f, "scp", remoteFile, path)
	},
}

func newLogFile(path, filename string) *os.File {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, os.ModePerm)
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	fullPath := fmt.Sprintf("%s/%s", path, strconv.Itoa(len(files)))
	os.Mkdir(fullPath, os.ModePerm)

	f, err := os.Create(fmt.Sprintf("%s/%s", fullPath, filename))

	if err != nil {
		log.Fatal(err)
	}

	return f
}

// init prepares the given machines to execute the benchmarks contained
// in the directory.
func (p platformRunner) init(f *os.File, nodeIDtoHostname map[int]string) {
	if len(nodeIDtoHostname) != 2 {
		fmt.Fprintf(f, "Requires 2 nodes\n")
		runtime.Goexit()
	}
	if _, err := os.Stat("./init.sh"); os.IsNotExist(err) {
		fmt.Fprintf(f, "./init.sh not present\n")
		runtime.Goexit()
	}
	if _, err := os.Stat("./scripts"); os.IsNotExist(err) {
		fmt.Fprintf(f, "./scripts not present\n")
		runtime.Goexit()
	}
	runCmd(f, "chmod", "-R", "a+x", "./scripts")
	runCmd(f, "zip", "-FSro", "./scripts.zip", "./scripts")

	fmt.Fprintf(f, "Putting and prepping scripts...\n")
	for _, dest := range nodeIDtoHostname {
		p.upload(f, dest, "scripts.zip")
		p.upload(f, dest, "init.sh")
		p.exec(f, dest, "chmod a+x init.sh")
		p.exec(f, dest, "./init.sh")
	}
	fmt.Fprintf(f, "Put and prepped scripts\n")
}

// run executes the benchmarks, downloads their artifacts, and then parses them.
func (p platformRunner) run(
	f *os.File,
	argVals map[string]string,
	nodeIDtoHostname map[int]string,
	resultsPath string,
) {
	if *loadOnly {
		fmt.Fprintf(f, "Skipping benchmarks\n")
		return
	}

	// checkForAllArgs would be more optimal placed elsewhere, given that
	// it only needs to be run once _but_ this is the best choke point to
	// ensure all future extensions of this program properly fill all of the
	// expected values.
	checkForAllArgs(f, argVals)

	fmt.Fprintf(f, "Running benchmarks for %s\n", resultsPath)

	for _, b := range benchmarks {
		if (*ioSkip && b.name == "io") || (*ioOnly && b.name != "io") {
			continue
		}

		fmt.Fprintf(f, "Running %s...\n", b.name)
		for _, r := range b.routines {
			if r.name != "" {
				fmt.Fprintf(f, "\tRunning %s...\n", r.name)
			}
			src, ok := nodeIDtoHostname[r.node]
			if !ok {
				log.Fatalf("%s: Invalid node number %d in routine %s; should be either 1 or 2\n", resultsPath, r.node, r.name)
			}
			cmd := fmt.Sprintf("%s %s", r.file, argVals[r.arg])
			if r.launchAsGoroutine {
				go p.exec(f, src, cmd)
			} else {
				p.exec(f, src, cmd)
			}
		}
		fmt.Fprintf(f, "Downloading artifacts for %s...\n", b.name)
		for _, art := range b.artifacts {
			p.download(f, nodeIDtoHostname[art.node], art.file, resultsPath)
		}
		fmt.Fprintf(f, "Downloaded artifacts for %s\n", b.name)
		fmt.Fprintf(f, "Finished %s\n", b.name)
	}
}

// parseResults converts the downloaded artifacts into CSVs.
func parseResults(f *os.File, dir string) {
	fmt.Fprintf(f, "Parsing results in %s\n", dir)
	runCmd(f, "./scripts/parse/parse-dir.sh", dir)
	fmt.Fprintf(f, "Done parsing results\n")
}

func convertArtifactFilenameToCSV(filename string) string {
	filenameToCSV := regexp.MustCompile("(\\/[a-z-]+)\\.log")
	logFilename := filenameToCSV.FindStringSubmatch(filename)
	return logFilename[1] + ".csv"
}

// uploadResults posts the CSVs in dir to Google sheets.
func uploadResults(f *os.File, dir string) {
	parseResults(f, dir)
	fmt.Fprintf(f, "Uploading results in %s...\n", dir)
	for _, b := range benchmarks {
		for _, a := range b.artifacts {
			fn := convertArtifactFilenameToCSV(a.file)
			appendDataToSpreadsheet(fn, dir)
		}
	}
	appendDataToSpreadsheet("/run-data.csv", dir)
	fmt.Fprintf(f, "Uploaded results\n")
}

// Some machine types are not available in the 4xlarge size. Add this option to
// machine sizes that are not 4xlarge. Note, that the machine still needs
// to support the specified options.
// For more information see: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-optimize-cpu.html#instance-specify-cpu-options
func getCpuOptions(machineType string) (awsCpuOptions string) {
	size := strings.Split(machineType, ".")[1]
	if size != "4xlarge" {
		awsCpuOptions = "--aws-cpu-options=CoreCount=8,ThreadsPerCore=2"
	}
	return
}

// roachprodRun creates a roachprod cluster, and then fully executes the benchmark suite.
func roachprodRun(cloudName, clusterPrefix, machineType string, ebs bool) {
	dateString := time.Now().Format("20060102")
	clusterName := fmt.Sprintf("%s-%s-%s-%s", clusterPrefix, cloudName, dateString, machineType)

	// Roachprod cluster names cannot contain dots; convert all of them to dashes.
	validClusterName := regexp.MustCompile(`[\.]`)
	clusterName = validClusterName.ReplaceAllString(clusterName, "-")

	fmt.Printf("\nChecking for existing cluster %s...\n", clusterName)
	if runCmdFindString(os.Stdout, clusterName, "roachprod", "list") {
		fmt.Println("Found existing cluster")
	} else {
		fmt.Printf("Creating new cluster...\n")
		// Create two machines with specified options (cloud, machine type, disk type for AWS)
		// using the steps outlined in `/deployment-steps.md`.
		switch cloudName {
		case "gcp":
			runCmd(os.Stdout, "roachprod", "create", clusterName, "-n", "2", "--gce-machine-type", machineType, "--gce-zones", "us-central1-a")
		case "aws":
			if ebs {
				runCmd(os.Stdout, "roachprod", "create", clusterName, "-n", "2", "--clouds", "aws", getCpuOptions(machineType), "--aws-machine-type", machineType, "--local-ssd=false", "--aws-ebs-volume-type", "io1", "--aws-ebs-iops", "20000")
			} else {
				runCmd(os.Stdout, "roachprod", "create", clusterName, "-n", "2", "--clouds", "aws", getCpuOptions(machineType), "--aws-machine-type-ssd", machineType)
			}
		default:
			log.Fatalf("Unsupported cloud option: %s", cloudName)
		}
		fmt.Println("Created cluster")
	}

	nodeIDtoHostname := map[int]string{
		1: clusterName + ":1",
		2: clusterName + ":2",
	}
	initlogPath := fmt.Sprintf("logs/%s/%s/%s/init", cloudName, machineType, dateString)
	initLog := newLogFile(initlogPath, "init.log")
	fmt.Printf("%s init log in %s\n", clusterName, initlogPath)
	roachprodRunner.init(initLog, nodeIDtoHostname)
	argVals := map[string]string{
		argCloudName:       cloudName,
		argNode2InternalIP: runCmdReturnString(initLog, "roachprod", "ip", clusterName+":2"),
	}

	runLogPath := fmt.Sprintf("logs/%s/%s/%s/run", cloudName, machineType, dateString)

	for i := 0; i < *iterations; i++ {
		// newLogFile does some very, very small amount of magic to generate a new directory
		// each time it's called, so you want to call it for each run.
		runLog := newLogFile(runLogPath, "run.log")
		fmt.Printf("%s run log now in %s\n", clusterName, runLogPath)
		resultsPath := createDir(runLog, cloudName, machineType)
		roachprodRunner.run(runLog, argVals, nodeIDtoHostname, resultsPath)

		uploadResults(runLog, resultsPath)
	}
}

// azureRun runs the benchmark suite on the provided machines.
// NOTE: azureRun is not a generic interface because it relies
// on an Azure-specific endpoint to get metadata about the machine.
func azureRun(username string) {
	if *node1IP == "" || *node2IP == "" {
		log.Fatal("Must pass in -node1 and -node2 IP addresses.")
	}

	if !isIPWellFormed(*node1IP) || !isIPWellFormed(*node2IP) {
		log.Fatal("-node1 or -node2 is invalid IP address")
	}

	node1 := fmt.Sprintf("%s@%s", username, *node1IP)
	node2 := fmt.Sprintf("%s@%s", username, *node2IP)

	nodeIDtoHostname := map[int]string{
		1: node1,
		2: node2,
	}

	dateString := time.Now().Format("20060102")

	initLog := newLogFile(fmt.Sprintf("logs/azure/%s/%s/init/", *node1IP, dateString), "init.log")

	shellRunner.init(initLog, nodeIDtoHostname)

	argVals := map[string]string{
		argCloudName:       "azure",
		argNode2InternalIP: runCmdReturnString(initLog, "ssh", node2, "./scripts/azure/get-internal-ip.sh"),
	}

	machineType := runCmdReturnString(initLog, "ssh", node1, "./scripts/azure/get-vm-type.sh")

	runLogPath := fmt.Sprintf("logs/azure/%s/%s/run/", *node1IP, dateString)

	for i := 0; i < *iterations; i++ {
		// newLogFile does some very, very small amount of magic to generate a new directory
		// each time it's called, so you want to call it for each run.
		runLog := newLogFile(runLogPath, "run.log")
		resultsPath := createDir(runLog, "azure", machineType)
		shellRunner.run(runLog, argVals, nodeIDtoHostname, resultsPath)

		uploadResults(runLog, resultsPath)
	}
}

// onPremRun runs on pre-provisioned VMs, but does not rely on any
// platform-specific features.
func onPremRun(username string) {
	if *node1IP == "" || *node2IP == "" {
		log.Fatal("Must pass in -node1 and -node2 IP addresses.")
	}

	if !isIPWellFormed(*node1IP) || !isIPWellFormed(*node2IP) {
		log.Fatal("-node1 or -node2 is invalid IP address")
	}

	node1 := fmt.Sprintf("%s@%s", username, *node1IP)
	node2 := fmt.Sprintf("%s@%s", username, *node2IP)

	nodeIDtoHostname := map[int]string{
		1: node1,
		2: node2,
	}

	dateString := time.Now().Format("20060102")

	var initLogPath, runLogPath, resultsDir string

	if *machineName == "" {
		initLogPath = fmt.Sprintf("logs/%s/%s/init/", *node1IP, dateString)
		runLogPath = fmt.Sprintf("logs/%s/%s/run/", *node1IP, dateString)
		resultsDir = *node1IP
	} else {
		initLogPath = fmt.Sprintf("logs/%s/%s/init/", *machineName, dateString)
		runLogPath = fmt.Sprintf("logs/%s/%s/init/", *machineName, dateString)
		resultsDir = *machineName
	}

	initWriter := newLogFile(initLogPath, "init.log")

	shellRunner.init(initWriter, nodeIDtoHostname)

	argVals := map[string]string{
		argCloudName: "on-prem",
	}

	if *node2InternalIP == "" {
		argVals[argNode2InternalIP] = runCmdReturnString(initWriter, "ssh", node2, "./scripts/on-prem/get-internal-ip.sh")

		if !isIPWellFormed(argVals[argNode2InternalIP]) {
			log.Fatal("Cannot automatically detect node 2 internal IP; please run again with -node2-internal=<node2's internal IP address>")
		}

		fmt.Fprintf(initWriter, "Node 2 internal IP address detected as %s", argVals[argNode2InternalIP])

	} else {
		argVals[argNode2InternalIP] = *node2InternalIP
	}

	for i := 0; i < *iterations; i++ {
		// newLogFile does some very, very small amount of magic to generate a new directory
		// each time it's called, so you want to call it for each run.
		runLog := newLogFile(runLogPath, "run.log")
		resultsPath := createDir(runLog, "on-prem", resultsDir)
		shellRunner.run(runLog, argVals, nodeIDtoHostname, resultsPath)
		fmt.Fprintf(runLog, "\n%d/%d iterations completed\n", i+1, *iterations)
	}
}

func main() {

	flag.Parse()

	if *iterations < 1 {
		log.Fatal("Iterations must be > 0")
	}

	_, err := exec.LookPath("pcregrep")
	if err != nil {
		log.Fatal("Install pcregrep in your $PATH (brew install pcre)")
	}

	username := *crlUsername

	if username == "" {
		username = runCmdReturnString(nil, "whoami")
	}

	if *runOnPrem {
		onPremRun(username)
		return
	}

	// Force login check before running any tests.
	_ = getSheetsClient()

	if *runAzure {
		azureRun(username)
		return
	}

	_, err = exec.LookPath("roachprod")
	if err != nil {
		log.Fatal("Install roachprod in your $PATH")
	}

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

	// Template for roachprod cluster prefixes.
	clusterPrefix := username + "-cldrprt19-micro"

	var wg sync.WaitGroup
	totalTests := 0
	for _, cloud := range clouds {
		totalTests += len(cloud.MachineTypes)
		totalTests += len(cloud.EBSMachineTypes)
	}

	wg.Add(totalTests)

	for _, cloud := range clouds {

		for _, machineType := range cloud.MachineTypes {
			go func(cloudName, machineType string) {
				defer wg.Done()
				roachprodRun(cloudName, clusterPrefix, machineType, false)
			}(cloud.Name, machineType)
		}

		for _, machineType := range cloud.EBSMachineTypes {
			if cloud.Name != "aws" {
				fmt.Printf("Only aws should have EBS machines, but have %s\n", cloud.Name)
			}
			go func(cloudName, machineType string) {
				defer wg.Done()
				roachprodRun(cloudName, clusterPrefix, machineType, true)
			}(cloud.Name, machineType)
		}
	}
	wg.Wait()
}
