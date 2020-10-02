package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var lifetime = flag.String("lifetime", "24h", "cluster lifetime")
var cloudDetailsFile = flag.String("cloud-details", "./cloudDetails/default.json", "run tests against specified input, which will be loaded into clouds")
var baseOutputDir = flag.String("output-dir", "./report-data", "directory to emit results and scripts")
var scriptsDir = flag.String("scripts-dir", "./scripts", "directory containing cloud benchmark scripts")
var reportVersion = flag.String("report-version", time.Now().Format("20060102"), "subdirectory to write data to")
var analyze = flag.Bool("analyze-results", false, "Analyze produced results")

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

function fetch_bench_tpcc_results() {
  echo "Implement me"
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

func (c CloudDetails) BasePath() string {
	return path.Join(*baseOutputDir, c.Cloud, c.Group)
}
func (c CloudDetails) ScriptDir() string {
	return path.Join(c.BasePath(), "scripts")
}
func (c CloudDetails) LogDir() string {
	return path.Join(c.BasePath(), "logs")
}

func ResultsFile(fname string, subdirs ...string) string {
	pieces := append([]string{*baseOutputDir, "results"}, subdirs...)
	p := path.Join(pieces...)
	if err := makeAllDirs(p); err != nil {
		panic(err)
	}
	return filepath.Join(p, fname)
}

func FormatMachineType(m string) string {
	return strings.Replace(m, ".", "-", -1)
}

func generateCloudScripts(cloud CloudDetails) error {
	basePath := path.Join(*baseOutputDir, cloud.Cloud, cloud.Group)
	scriptDir := path.Join(basePath, "scripts")

	if err := makeAllDirs(basePath, scriptDir, cloud.LogDir()); err != nil {
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
				fmt.Fprintf(buf, "=%q", val)
			}
		}
		templateArgs.EvaledArgs = buf.String()

		scriptName := path.Join(
			scriptDir,
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

func generateScripts(clouds []CloudDetails) {
	log.Printf("Generating scripts under %s", *baseOutputDir)
	if err := os.MkdirAll(*baseOutputDir, 0755); err != nil {
		log.Fatal(err)
	}

	// Generate scripts.
	for _, cloud := range clouds {
		if err := generateCloudScripts(cloud); err != nil {
			log.Fatal(err)
		}
	}
}

// resultsAnalyzer is an interface responsible for analyzing benchmark results.
type resultsAnalyzer interface {
	io.Closer
	Analyze(cloud CloudDetails) error
}

// lat represents fio total latencies.
// Values in nanoseconds.
type lat struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
	Dev  float64 `json:"stddev"`
}

// clat represents completion latencies.
type clat struct {
	lat
	Percentiles map[string]int64 `json:"percentile"`
}

type ioStats struct {
	TotalIOS  int64 `json:"total_ios"` // Total # Of IOs
	IOBytes   int64 `json:"io_bytes"`  // Total size of IO
	RuntimeMS int64 `json:"runtime"`   // Duration (msec)
	Lat       lat   `json:"lat_ns"`    // IO latencies
	Clat      clat  `json:"clat_ns"`   // IO completion latencies. includes percentiles
}

func ioStatsCSV(s *ioStats) []string {
	secs := float64(s.RuntimeMS) / 1000
	rate := func(v int64) float64 {
		if v > 0 {
			return float64(v) / secs
		}
		return 0
	}

	fields := []string{
		// Number and rate of IO operations.
		fmt.Sprintf("%d", s.TotalIOS),
		fmt.Sprintf("%.3f", rate(s.TotalIOS)), // IOP/sec
		// Total amount of data read or written + Bandwidth in KiB/s
		fmt.Sprintf("%d", s.IOBytes),
		fmt.Sprintf("%f", rate(s.IOBytes)/1024), // Bandwidth: KiB/s
		// Total Latency
		fmt.Sprintf("%f", s.Lat.Min),
		fmt.Sprintf("%f", s.Lat.Max),
		fmt.Sprintf("%f", s.Lat.Mean),
		fmt.Sprintf("%f", s.Lat.Dev),
	}

	// Add completion latency percentiles.
	for _, pct := range []string{"90.000000", "95.000000", "99.000000", "99.900000", "99.990000"} {
		fields = append(fields, fmt.Sprintf("%d", s.Clat.Percentiles[pct]))
	}
	return fields
}

type fioJob struct {
	Name       string             `json:"jobname"`
	ReadStats  ioStats            `json:"read"`
	WriteStats ioStats            `json:"write"`
	LatNS      map[string]float64 `json:"latency_ns"`
	LatUS      map[string]float64 `json:"latency_us"`
	LatMS      map[string]float64 `json:"latency_ms"`
}

type fioResults struct {
	Timestamp int64    `json:"timestamp"`
	Jobs      []fioJob `json:"jobs"`
}

const fioResultsCSVHeader = `Cloud,Group,Machine,Date,Job,RdIOPs,RdIOP/s,RdBytes,RdBW(KiB/s),RdlMin,RdlMax,RdlMean,RdlStd,Rd90,Rd95,Rd99,Rd99.9,Rd99.99,WrIOPs,WrIOP/s,WrBytes,WrBW(KiB/s),WrlMin,WrlMax,WrlMean,WrlStd,Wr90,Wr95,Wr99,Wr99.9,Wr99.99,`

func (r *fioResults) CSV(cloud CloudDetails, machineType string, wr io.Writer) {
	for _, j := range r.Jobs {
		fields := []string{
			cloud.Cloud,
			cloud.Group,
			machineType,
			time.Unix(r.Timestamp, 0).String(),
			j.Name,
		}
		fields = append(fields, ioStatsCSV(&j.ReadStats)...)
		fields = append(fields, ioStatsCSV(&j.WriteStats)...)
		fmt.Fprintf(wr, "%s\n", strings.Join(fields, ","))
	}
}

func analyzeFIO(cloud CloudDetails, machineType string, wr io.Writer) error {
	// Find successful FIO runs (those that have success file)
	glob := path.Join(cloud.LogDir(), FormatMachineType(machineType), "fio-results.*/success")
	goodRuns, err := filepath.Glob(glob)
	if err != nil {
		return err
	}

	for _, r := range goodRuns {
		// Read fio-results
		log.Printf("Analyzing %s", r)
		data, err := ioutil.ReadFile(path.Join(filepath.Dir(r), "fio-results.json"))
		if err != nil {
			return err
		}
		res := &fioResults{}
		if err := json.Unmarshal(data, res); err != nil {
			return err
		}
		res.CSV(cloud, machineType, wr)
	}
	return nil
}

type analyzeFn func(c CloudDetails, machineType string) error

func forEachMachine(cloud CloudDetails, fn analyzeFn) error {
	for machineType := range cloud.MachineTypes {
		if err := fn(cloud, machineType); err != nil {
			return err
		}
	}
	return nil
}

type fioAnalyzer struct {
}

var _ resultsAnalyzer = &fioAnalyzer{}

func newFioAnalyzer() *fioAnalyzer {
	return &fioAnalyzer{}
}

func (f *fioAnalyzer) Close() error {
	return nil
}

func (f *fioAnalyzer) Analyze(cloud CloudDetails) error {
	// FIO emits results per group.
	wr, err := os.OpenFile(ResultsFile("fio.csv", cloud.Cloud, cloud.Group),
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer wr.Close()

	fmt.Fprintf(wr, "%s\n", fioResultsCSVHeader)

	return forEachMachine(cloud, func(details CloudDetails, machineType string) error {
		err = analyzeFIO(details, machineType, wr)
		return err
	})
}

//
// CPU Analysis
//
type coremarkResult struct {
	cores   int64
	single  float64
	multi   float64
	modtime time.Time
}

type coremarkAnalyzer struct {
	cloud          string
	machineResults map[string]*coremarkResult
}

var _ resultsAnalyzer = &coremarkAnalyzer{}

func newCoremarkAnalyzer() *coremarkAnalyzer {
	return &coremarkAnalyzer{machineResults: make(map[string]*coremarkResult)}
}

func parseCoremarkLog(p string) (int64, float64, error) {
	// Extract the last line for the coremark output, and emit itersations/sec as well
	// as (optional) number of cores that were used when running this benchmark.
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("tail  -1 -q %s |cut -d/ -f1,4 | cut -d: -f2", p))
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	pieces := strings.Split(string(out), "/")
	if len(pieces) == 0 || len(pieces) > 2 {
		return 0, 0, fmt.Errorf("expected up to 2 fields, found 0 in %q", p)
	}

	iters, err := strconv.ParseFloat(strings.TrimSpace(pieces[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("error parsing %q in %s: %v", pieces[0], p, err)
	}
	var cores int64 = 1
	if len(pieces) == 2 {
		c, err := strconv.ParseInt(strings.TrimSpace(pieces[1]), 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("error parsing %q in %s: %v", pieces[1], p, err)
		}
		cores = c
	}
	return cores, iters, nil
}

func (c *coremarkAnalyzer) analyzeCPU(cloud CloudDetails, machineType string) error {
	// Find successful Coremark runs (those that have success file)
	glob := path.Join(cloud.LogDir(), FormatMachineType(machineType), "coremark-results.*/success")
	goodRuns, err := filepath.Glob(glob)
	if err != nil {
		return err
	}

	parseLogs := func(glob string) (int64, float64, error) {
		runs, err := filepath.Glob(glob)
		if err != nil {
			return 0, 0, err
		}

		var cores int64
		var totalIters float64
		for _, run := range runs {
			nc, iters, err := parseCoremarkLog(run)
			if err != nil {
				return 0, 0, err
			}
			if cores == 0 {
				cores = nc
			} else if cores != nc {
				return 0, 0, fmt.Errorf("expected same number of cores (%d), found %d in %q", cores, nc, run)
			}
			totalIters += iters
		}
		return cores, totalIters / float64(len(runs)), nil
	}

	for _, r := range goodRuns {
		// Read coremark-results
		log.Printf("Analyzing %s", r)
		info, err := os.Stat(r)
		if err != nil {
			return err
		}

		if res, ok := c.machineResults[machineType]; ok && res.modtime.Sub(info.ModTime()) > 0 {
			log.Printf("Skipping coremark log %q (already analyzed newer)", r)
			continue
		}

		_, single, err := parseLogs(path.Join(filepath.Dir(r), "single-*.log"))
		if err != nil {
			return err
		}
		cores, multi, err := parseLogs(path.Join(filepath.Dir(r), "multi-*.log"))
		if err != nil {
			return err
		}
		c.machineResults[machineType] = &coremarkResult{
			cores:   cores,
			single:  single,
			multi:   multi,
			modtime: info.ModTime(),
		}
	}
	return nil
}

func (c *coremarkAnalyzer) Analyze(cloud CloudDetails) error {
	if len(c.cloud) == 0 {
		c.cloud = cloud.Cloud
	} else if cloud.Cloud != c.cloud {
		return fmt.Errorf("expected %s cloud, got %s", c.cloud, cloud.Cloud)
	}

	return forEachMachine(cloud, func(details CloudDetails, machineType string) error {
		return c.analyzeCPU(details, machineType)
	})
}

func (c *coremarkAnalyzer) Close() (err error) {
	f, err := os.OpenFile(ResultsFile("cpu.csv", c.cloud), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer func() { err = f.Close() }()

	fmt.Fprint(f, "Cloud,Date,MachineType,Cores,Single,Multi,Multi/vCPU\n")
	for machineType, result := range c.machineResults {
		fields := []string{
			c.cloud,
			result.modtime.String(),
			machineType,
			fmt.Sprintf("%d", result.cores),
			fmt.Sprintf("%f", result.single),
			fmt.Sprintf("%f", result.multi),
			fmt.Sprintf("%f", result.multi/float64(result.cores)),
		}
		fmt.Fprintf(f, "%s\n", strings.Join(fields, ","))
	}
	return
}

const netCSVHeader = ``

type netAnalyzer struct {
}

func newNetAnalyzer() *netAnalyzer {
	return &netAnalyzer{}
}

func (n *netAnalyzer) Close() error {
	log.Print("net: Close() unimplemented\n")
	return nil
}

func (n *netAnalyzer) Analyze(cloud CloudDetails) error {
	log.Print("net: Analyze() unimplemented\n")
	return nil
}

var _ resultsAnalyzer = &netAnalyzer{}

func analyzeResults(clouds []CloudDetails) {
	cpu := newCoremarkAnalyzer()
	defer cpu.Close()

	net := newNetAnalyzer()
	defer net.Close()

	fio := newFioAnalyzer()
	defer fio.Close()

	// Generate scripts.
	for _, cloud := range clouds {
		if err := cpu.Analyze(cloud); err != nil {
			panic(err)
		}
		if err := net.Analyze(cloud); err != nil {
			panic(err)
		}
		if err := fio.Analyze(cloud); err != nil {
			panic(err)
		}
	}
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

	if *analyze {
		analyzeResults(clouds)
	} else {
		generateScripts(clouds)
	}
}
