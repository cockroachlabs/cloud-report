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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/spf13/cobra"
)

var reportVersion string
var baseOutputDir string

type machineConfig struct {
	RoachprodArgs map[string]string `json:"roachprodArgs"`
	BenchArgs     map[string]string `json:"benchArgs"`
}

// CloudDetails provides the name of the cloud and the different
// machine types you should run the benchmark suite against.
type CloudDetails struct {
	Cloud string `json:"cloud"`
	Group string `json:"group"`

	// Common arguments passed to roachprod create.
	RoachprodArgs map[string]string `json:"roachprodArgs"`

	// Map from machine type to the map of the machine specific arguments
	// that should be passed when creating cluster.
	MachineTypes map[string]machineConfig `json:"machineTypes"`
}

func (c CloudDetails) BasePath() string {
	return path.Join(baseOutputDir, reportVersion, c.Cloud, c.Group)
}

func (c CloudDetails) ScriptDir() string {
	return path.Join(c.BasePath(), "scripts")
}

func (c CloudDetails) LogDir() string {
	return path.Join(c.BasePath(), "logs")
}

type cloudsValue struct {
	c *[]CloudDetails
}

func (cv *cloudsValue) Type() string {
	return "JSON"
}

func (cv *cloudsValue) String() string {
	return ""
}

func (cv *cloudsValue) Set(s string) error {
	// Parse cloud configuration file
	f, err := os.Open(s)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := ioutil.ReadAll(f)

	var clouds []CloudDetails
	err = json.Unmarshal(b, &clouds)
	if err != nil {
		return err
	}
	*cv.c = append(*cv.c, clouds...)
	return nil
}

var clouds []CloudDetails

func newCloudsValue(c *[]CloudDetails) *cloudsValue {
	return &cloudsValue{c}
}

func makeAllDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cloud-report",
	Short: "Execute CockroachLabs Cloud Report",
	Long:  `Execution and data analysis for CockroachLabs cloud report`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&reportVersion, "report-version", "r",
		time.Now().Format("20060102"), "subdirectory for cloud report data")
	rootCmd.PersistentFlags().StringVarP(&baseOutputDir, "output-dir", "o",
		"./report-data", "directory to emit results and scripts")
	rootCmd.PersistentFlags().VarP(newCloudsValue(&clouds), "cloud-details", "d",
		"path(s) to JSON file containing cloud specific configuration.")
	_ = rootCmd.MarkPersistentFlagRequired("cloud-details")
}
