/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/silogen/cluster-bloom/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "bloom",
	Short: "Cluster-Bloom creates a cluster",
	Long: `
Cluster-Bloom installs and configures a Kubernetes cluster.
It installs ROCm and other needed settings to prepare a (primarily AMD GPU) node to be part of a Kubernetes cluster,
and ready to be deployed with Cluster-Forge.

Available Configuration Variables:
  - FIRST_NODE: Set to true if this is the first node in the cluster (default: true).
  - GPU_NODE: Set to true if this node has GPUs (default: true).
  - OIDC_URL: The URL of the OIDC provider (default: "").
  - SERVER_IP: The IP address of the RKE2 server (required for additional nodes).
  - JOIN_TOKEN: The token used to join additional nodes to the cluster (required for additional nodes).
  - SKIP_DISK_CHECK: Set to true to skip disk-related operations (default: false).
  - LONGHORN_DISKS: Comma-separated list of disk paths to use for Longhorn (default: "").
  - ONEPASS_CONNECT_TOKEN: The token used for 1Password Connect integration (default: "").

Usage:
  Use the --config flag to specify a configuration file, or set the above variables in the environment or a Viper-compatible config file.
`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Starting package installation")
		rootSteps()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.AddCommand(helpCmd)
}

func initConfig() {
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			log.Fatalf("Config file does not exist: %s", cfgFile)
		}
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Could not determine home directory: %v", err)
		}
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".bloom")
	}

	viper.SetDefault("FIRST_NODE", true)
	viper.SetDefault("GPU_NODE", true)
	viper.SetDefault("OIDC_URL", "")
	viper.SetDefault("SKIP_DISK_CHECK", false)
	viper.SetDefault("LONGHORN_DISKS", "")
	viper.SetDefault("ONEPASS_CONNECT_TOKEN", "")

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	requiredConfigs := []string{"FIRST_NODE", "GPU_NODE"}
	for _, config := range requiredConfigs {
		if !viper.IsSet(config) {
			log.Fatalf("Required configuration item '%s' is not set", config)
		}
	}

	if !viper.GetBool("FIRST_NODE") {
		requiredConfigs := []string{"SERVER_IP", "JOIN_TOKEN"}
		for _, config := range requiredConfigs {
			if !viper.IsSet(config) {
				log.Fatalf("Required configuration item '%s' is not set", config)
			}
		}
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	currentDir, err := os.Getwd()
	if err != nil {
		log.Warnf("Could not determine current directory: %v", err)
		return
	}

	logPath := filepath.Join(currentDir, "bloom.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Warnf("Could not open log file: %v", err)
		return
	}
	log.SetOutput(logFile)
	logConfigValues()
}

func logConfigValues() {
	log.Info("Configuration values:")
	for _, key := range viper.AllKeys() {
		value := viper.Get(key)
		if key == "join_token" {
			value = "---redacted---"
		}
		log.Infof("%s: %v", key, value)
	}
}

func rootSteps() {
	preK8Ssteps := []pkg.Step{
		pkg.CheckUbuntuStep,
		pkg.HasSufficientRootPartitionStep,
		pkg.NVMEDrivesAvailableStep,
		pkg.InstallDependentPackagesStep,
		pkg.UninstallRKE2Step,
		pkg.CleanDisksStep,
		pkg.SetupMultipathStep,
		pkg.UpdateModprobeStep,
		pkg.SelectDrivesStep,
		pkg.MountSelectedDrivesStep,
		pkg.PrepareRKE2Step,
		pkg.GenerateLonghornDiskStringStep,
		pkg.InstallK8SToolsStep,
		pkg.InotifyInstancesStep,
		pkg.SetupAndCheckRocmStep,
		pkg.OpenPortsStep,
		pkg.SetRenderGroupStep,
	}
	k8Ssteps := []pkg.Step{
		pkg.SetupRKE2Step,
	}
	postK8Ssteps := []pkg.Step{
		pkg.SetupManifestsStep,
		pkg.CreateMetalLBConfigStep,
		pkg.SetupKubeConfig,
		pkg.SetupClusterForgeStep,
	}

	if viper.IsSet("ONEPASS_CONNECT_TOKEN") && viper.GetString("ONEPASS_CONNECT_TOKEN") != "" {
		postK8Ssteps = append(postK8Ssteps, pkg.SetupOnePasswordSecretStep)
	}

	postK8Ssteps = append(postK8Ssteps, pkg.FinalOutput)

	pkg.RunStepsWithUI(append(append(preK8Ssteps, k8Ssteps...), postK8Ssteps...))
}

func displayHelp() {
	helpContent := `
Cluster-Bloom Help:

Available Configuration Variables:
  - FIRST_NODE: Set to true if this is the first node in the cluster (default: true).
  - GPU_NODE: Set to true if this node has GPUs (default: true).
  - OIDC_URL: The URL of the OIDC provider (default: "").
  - SERVER_IP: The IP address of the RKE2 server (required for additional nodes).
  - JOIN_TOKEN: The token used to join additional nodes to the cluster (required for additional nodes).
  - SKIP_DISK_CHECK: Set to true to skip disk-related operations (default: false).
  - LONGHORN_DISKS: Comma-separated list of disk paths to use for Longhorn (default: "").
  - ONEPASS_CONNECT_TOKEN: The token used for 1Password Connect integration (default: "").

Usage:
  Use the --config flag to specify a configuration file, or set the above variables in the environment or a Viper-compatible config file.
`
	fmt.Println(helpContent)
}

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Display help information",
	Run: func(cmd *cobra.Command, args []string) {
		displayHelp()
	},
}
