/*
Copyright © 2019 Doppler <support@doppler.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	api "doppler-cli/api"
	configuration "doppler-cli/config"
	dopplerErrors "doppler-cli/errors"
	"doppler-cli/utils"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
)

var deployHost string
var key string
var project string
var config string

var runCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "A brief description of your command",
	Long: `Run a command with secrets injected into the environment

Usage:
doppler run printenv
doppler run -- printenv
doppler run --key=123 -- printenv`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			dopplerErrors.CommandMissingArgument(cmd)
		}

		silent := utils.GetBoolFlag(cmd, "silent")

		fallbackReadonly := utils.GetBoolFlag(cmd, "fallback-readonly")
		fallbackOnly := utils.GetBoolFlag(cmd, "fallback-only")
		fallbackPath := utils.GetFilePath(cmd.Flag("fallback").Value.String(), "")

		if cmd.Flags().Changed("fallback") && fallbackPath == "" {
			utils.Err(errors.New("invalid fallback file path"))
		}

		localConfig := configuration.LocalConfig(cmd)
		secrets := getSecrets(cmd, localConfig, fallbackPath, fallbackReadonly, fallbackOnly)

		env := os.Environ()
		excludedKeys := []string{"PATH", "PS1", "HOME"}
		for name, value := range secrets {
			addKey := true
			for _, excludedKey := range excludedKeys {
				if excludedKey == name {
					addKey = false
					break
				}
			}

			if addKey {
				env = append(env, fmt.Sprintf("%s=%s", name, value))
			}
		}

		err := utils.RunCommand(args, env, !silent)
		if err != nil {
			fmt.Println(fmt.Sprintf("Error trying to execute command: %s", args))
			utils.Err(err)
		}
	},
}

func getSecrets(cmd *cobra.Command, localConfig configuration.ScopedConfig, fallbackPath string, fallbackReadonly bool, fallbackOnly bool) map[string]string {
	useFallbackFile := (fallbackPath != "")
	if useFallbackFile && fallbackOnly {
		return readFallbackFile(fallbackPath)
	}

	response, err := api.GetDeploySecrets(cmd, localConfig.Key.Value, localConfig.Project.Value, localConfig.Config.Value)

	if !useFallbackFile && err != nil {
		utils.Err(err)
	}

	if useFallbackFile {
		if err != nil {
			return readFallbackFile(fallbackPath)
		}

		if !fallbackReadonly {
			err := ioutil.WriteFile(fallbackPath, response, 0600)
			if err != nil {
				fmt.Println("Unable to write fallback file")
				utils.Err(err)
			}
		}
	}

	secrets, err := api.ParseDeploySecrets(response)
	if err != nil {
		fmt.Println("Unable to parse response")
		utils.Err(err)
	}

	return secrets
}

func readFallbackFile(path string) map[string]string {
	fmt.Println("Using fallback file")
	response, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("Unable to read fallback file")
		utils.Err(err)
	}

	secrets, err := api.ParseDeploySecrets(response)
	if err != nil {
		fmt.Println("Unable to parse fallback file")
		utils.Err(err)
	}

	return secrets
}

func init() {
	runCmd.Flags().Bool("silent", false, "don't output the response")
	runCmd.Flags().String("project", "", "doppler project (e.g. backend)")
	runCmd.Flags().String("config", "", "doppler config (e.g. dev)")

	runCmd.Flags().String("fallback", "", "write secrets to this file after connecting to Doppler. secrets will be read from this file if future connection attempts are unsuccessful.")
	runCmd.Flags().Bool("fallback-readonly", false, "don't update or modify the fallback file")
	runCmd.Flags().Bool("fallback-only", false, "don't request secrets from Doppler. all secrets will be read directly from the fallback file")

	rootCmd.AddCommand(runCmd)
}