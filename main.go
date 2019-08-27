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
	"regexp"
	"strconv"
	"strings"
	"time"
)

var runAzure = flag.Bool("azure", false, "run microbenchmarks on Azure VMs you've already provisioned.")
var skipIO = flag.Bool("skipio", false, "skip the IO tests, which take a long time to run.")
var iterations = flag.Int("iterations", 1, "run the benchmarks on the same machines {iterations} number of times.")
var cloudDetailsFile = flag.String("cloudDetails", "./cloudDetails/default.json", "run tests against specified input, which will be loaded into clouds")
var azureNode1ip = flag.String("node1", "", "IP address of node 1")
var azureNode2ip = flag.String("node2", "", "IP address of node 2")

const (
	argNode2IP   = "node2IP"
	argCloudName = "cloudName"
)

type benchmarkRoutine struct {
	name              string // Optional name to print when routine is running.
	file              string // Script containing routine.
	arg               string // Name of argument to pass.
	launchAsGoroutine bool   // Launch routine as goroutine.
	node              int    // which node should run this.
}
type benchmark struct {
	name      string             // Name to print when benchmark is running.
	routines  []benchmarkRoutine // routine to run.
	artifacts []artifact         // artifacts to donwload at end of run.
}
type artifact struct {
	file string
	node int
}

var benchmarks = []benchmark{
	{
		name: "ping",
		routines: []benchmarkRoutine{{
			file: "./scripts/gen/network-ping.sh",
			arg:  argNode2IP,
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
				arg:  argNode2IP,
				node: 1,
			},
		},
		artifacts: []artifact{
			{"~/network-iperf-client.log", 1},
		},
	},
	{
		name: "io",
		routines: []benchmarkRoutine{{
			file: "./scripts/gen/io.sh",
			arg:  argCloudName,
			node: 1,
		}},
		artifacts: []artifact{
			{"/mnt/data1/IO_LOAD_results.log", 1},
			{"/mnt/data1/IO_WR_results.log", 1},
			{"/mnt/data1/IO_RD_results.log", 1},
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
func runCmd(name string, arg ...string) string {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(name, arg...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Died on command", cmd)
		fmt.Println(stdout.String())
		fmt.Println(stderr.String())
		log.Fatal(err)
	}
	return stdout.String()
}

// runCmdReturnString attempts to clean and return the command's
// stdout.
func runCmdReturnString(name string, arg ...string) string {
	strVal := runCmd(name, arg...)
	strVal = strings.Replace(strVal, "\"", "", -1)
	return strings.TrimRight(strVal, "\n")
}

// runCmdFindString searches a command's output for the given string
// and confirms its presence or absence.
func runCmdFindString(searchString, name string, arg ...string) bool {
	strVal := runCmd(name, arg...)

	stringFound := strings.Contains(strVal, searchString)

	return stringFound
}

// createDir is a convenience function to create a directory using the
// naming schema that we've determined for the cloud report.
func createDir(cloudName, machineType string) string {
	dateString := time.Now().Format("20060102")
	resultsParentPath := fmt.Sprintf("results/%s/%s/%s", cloudName, machineType, dateString)
	if _, err := os.Stat(resultsParentPath); os.IsNotExist(err) {
		os.MkdirAll(resultsParentPath, os.ModePerm)
	}
	files, err := ioutil.ReadDir(resultsParentPath)
	if err != nil {
		log.Fatal(err)
	}
	resultsPath := fmt.Sprintf("%s/%s", resultsParentPath, strconv.Itoa(len(files)))
	os.Mkdir(resultsPath, os.ModePerm)

	return resultsPath
}

// platformRunner lets you describe methods that allow arbitrary platforms
// upload, run, and download files.
type platformRunner struct {
	uploader   func(dest, file string)
	runner     func(src, cmd string)
	downloader func(src, file, path string)
}

var roachprodRunner = platformRunner{
	uploader: func(dest, file string) {
		runCmd("roachprod", "put", dest, file)
	},
	runner: func(src, cmd string) {
		runCmd("roachprod", "run", src, cmd)
	},
	downloader: func(src, file, path string) {
		runCmd("roachprod", "get", src, file, path)
	},
}

var azureRunner = platformRunner{
	uploader: func(dest, file string) {
		rootOfDest := dest + ":~"
		runCmd("scp", file, rootOfDest)
	},
	runner: func(src, cmd string) {
		runCmd("ssh", src, cmd)
	},
	downloader: func(src, file, path string) {
		remoteFile := fmt.Sprintf("%s:%s", src, file)
		runCmd("scp", remoteFile, path)
	},
}

// init prepares the given machines to execute the benchmarks contained
// in the directory.
func (p platformRunner) init(nodeIDtoHostname map[int]string) {
	if len(nodeIDtoHostname) != 2 {
		log.Fatal("Requires 2 nodes")
	}
	if _, err := os.Stat("./init.sh"); os.IsNotExist(err) {
		log.Fatal("./init.sh not present")
	}
	if _, err := os.Stat("./scripts"); os.IsNotExist(err) {
		log.Fatal("./scripts not present")
	}
	runCmd("chmod", "-R", "a+x", "./scripts")
	runCmd("zip", "-FSro", "./scripts.zip", "./scripts")

	fmt.Println("Putting and prepping scripts...")
	for _, dest := range nodeIDtoHostname {
		p.uploader(dest, "scripts.zip")
		p.uploader(dest, "init.sh")
		p.runner(dest, "chmod a+x init.sh")
		p.runner(dest, "./init.sh")
	}
	fmt.Println("Put and prepped scripts")
}

// run executes the benchmarks, downloads their artifacts, and then parses them.
func (p platformRunner) run(
	argVals map[string]string,
	nodeIDtoHostname map[int]string,
	resultsPath string,
) {
	fmt.Printf("Running benchmarks for %s\n", resultsPath)
	for _, b := range benchmarks {
		if *skipIO && b.name == "io" {
			continue
		}
		fmt.Printf("Running %s...\n", b.name)
		for _, r := range b.routines {
			if r.name != "" {
				fmt.Printf("\tRunning %s...\n", r.name)
			}
			src, ok := nodeIDtoHostname[r.node]
			if !ok {
				log.Fatalf("Invalid node number %d in routine %s; should be either 1 or 2\n", r.node, r.name)
			}
			cmd := fmt.Sprintf("%s %s", r.file, argVals[r.arg])
			if r.launchAsGoroutine {
				go p.runner(src, cmd)
			} else {
				p.runner(src, cmd)
			}
		}
		fmt.Printf("Downloading artifacts for %s...\n", b.name)
		for _, art := range b.artifacts {
			p.downloader(nodeIDtoHostname[art.node], art.file, resultsPath)
		}
		fmt.Printf("Downloaded artifacts for %s\n", b.name)
		fmt.Printf("Finished %s\n", b.name)
	}

	parseResults(resultsPath)
}

// parseResults converts the downloaded artifacts into CSVs.
func parseResults(dir string) {
	fmt.Printf("Parsing results in %s\n", dir)
	runCmd("./scripts/parse/parse-dir.sh", dir)
	fmt.Printf("Done parsing results\n")
}

// roachprodRun creates a roachprod cluster, and then fully executes the benchmark suite.
func roachprodRun(cloudName, clusterPrefix, machineType string, ebs bool) {
	dateString := time.Now().Format("20060102")
	var clusterName string
	if ebs {
		clusterName = fmt.Sprintf("%s-%s-%s-%s-ebs", clusterPrefix, cloudName, dateString, machineType)
	} else {
		clusterName = fmt.Sprintf("%s-%s-%s-%s", clusterPrefix, cloudName, dateString, machineType)
	}

	// Roachprod cluster names cannot contain dots; convert all of them to dashes.
	validClusterName := regexp.MustCompile(`[\.]`)
	clusterName = validClusterName.ReplaceAllString(clusterName, "-")

	fmt.Printf("\nChecking for existing cluster %s...\n", clusterName)
	if runCmdFindString(clusterName, "roachprod", "list") {
		fmt.Println("Found existing cluster")
	} else {
		fmt.Printf("Creating new cluster...\n")
		// Create two machines with specified options (cloud, machine type, disk type for AWS)
		// using the steps outlined in `/deployment-steps.md`.
		switch cloudName {
		case "gcp":
			runCmd("roachprod", "create", clusterName, "-n", "2", "--gce-machine-type", machineType)
		case "aws":
			if ebs {
				runCmd("roachprod", "create", clusterName, "-n", "2", "--aws-machine-type", machineType, "--local-ssd=false")
			} else {
				runCmd("roachprod", "create", clusterName, "-n", "2", "--aws-machine-type-ssd", machineType)
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
	roachprodRunner.init(nodeIDtoHostname)

	argVals := map[string]string{
		argCloudName: cloudName,
		argNode2IP:   runCmdReturnString("roachprod", "ip", clusterName+":2"),
	}

	for i := 0; i < *iterations; i++ {
		var resultsPath string
		if ebs {
			resultsPath = createDir(cloudName, machineType+"-ebs")
		} else {
			resultsPath = createDir(cloudName, machineType)
		}
		roachprodRunner.run(argVals, nodeIDtoHostname, resultsPath)
	}
}

// azureRun runs the benchmark suite on the provided machines.
// NOTE: azureRun is not a generic interface because it relies
// on an Azure-specific endpoint to get metadata about the machine.
// However, the scripts in the azure directory could be made into
// generic scripts that check the endpoint of all known providers
// for those details.
func azureRun(username string) {
	if *azureNode1ip == "" || *azureNode2ip == "" {
		log.Fatal("Must pass in -node1 and -node2 IP addresses.")
	}

	node1 := fmt.Sprintf("%s@%s", username, *azureNode1ip)
	node2 := fmt.Sprintf("%s@%s", username, *azureNode2ip)

	nodeIDtoHostname := map[int]string{
		1: node1,
		2: node2,
	}

	azureRunner.init(nodeIDtoHostname)

	argVals := map[string]string{
		argCloudName: "azure",
		argNode2IP:   runCmdReturnString("ssh", node2, "./scripts/azure/get-internal-ip.sh"),
	}

	machineType := runCmdReturnString("ssh", node1, "./scripts/azure/get-vm-type.sh")

	for i := 0; i < *iterations; i++ {
		resultsPath := createDir("azure", machineType)
		azureRunner.run(argVals, nodeIDtoHostname, resultsPath)
	}
}

func main() {
	flag.Parse()

	if *iterations < 1 {
		log.Fatal("Iterations must be > 0")
	}

	username := runCmdReturnString("whoami")

	if *runAzure {
		azureRun(username)
		return
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

	_, err = exec.LookPath("roachprod")
	if err != nil {
		log.Fatal("Install roachprod in your $PATH")
	}

	// Template for roachprod cluster prefixes
	clusterPrefix := username + "-cldrprt19-micro"

	for _, cloud := range clouds {

		for _, machineType := range cloud.MachineTypes {
			roachprodRun(cloud.Name, clusterPrefix, machineType, false)
		}

		for _, machineType := range cloud.EBSMachineTypes {
			if cloud.Name != "aws" {
				fmt.Printf("Only aws should have EBS machines, but have %s\n", cloud.Name)
			}
			roachprodRun(cloud.Name, clusterPrefix, machineType, true)
		}
	}
}
