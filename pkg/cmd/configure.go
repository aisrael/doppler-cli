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
	"errors"
	"fmt"
	"strings"

	"github.com/DopplerHQ/cli/pkg/configuration"
	"github.com/DopplerHQ/cli/pkg/models"
	"github.com/DopplerHQ/cli/pkg/printer"
	"github.com/DopplerHQ/cli/pkg/utils"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "View the config file",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		all := utils.GetBoolFlag(cmd, "all")
		jsonFlag := utils.OutputJSON

		if all {
			printer.Configs(configuration.AllConfigs(), jsonFlag)
			return
		}

		scope := cmd.Flag("scope").Value.String()
		config := configuration.Get(scope)
		printer.ScopedConfig(config, jsonFlag)
	},
}

var configureDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "View active configuration utilizing all config sources",
	Long: `View active configuration utilizing all config sources.

This prints the active configuration that will be used by other CLI commands.
This factors in command line flags (--token=123), environment variables (DOPPLER_TOKEN=123),
and the config file (in order from highest to lowest precedence)`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		jsonFlag := utils.OutputJSON

		config := configuration.LocalConfig(cmd)
		printer.ScopedConfigSource(config, "", jsonFlag, true)
	},
}

var configureGetCmd = &cobra.Command{
	Use:   "get [options]",
	Short: "Get the value of one or more options in the config file",
	Long: `Get the value of one or more options in the config file.

Ex: output the options "key" and "otherkey":
doppler configure get key otherkey`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("requires at least 1 arg(s), only received 0")
		}

		for _, arg := range args {
			if !configuration.IsValidConfigOption(arg) {
				return errors.New("invalid option " + arg)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		jsonFlag := utils.OutputJSON
		plain := utils.GetBoolFlag(cmd, "plain")

		scope := cmd.Flag("scope").Value.String()
		conf := configuration.Get(scope)

		if plain {
			sbEmpty := true
			var sb strings.Builder

			for _, arg := range args {
				value, _ := configuration.GetScopedConfigValue(conf, arg)
				if sbEmpty {
					sbEmpty = false
				} else {
					sb.WriteString("\n")
				}

				sb.WriteString(value)
			}

			fmt.Println(sb.String())
			return
		}

		if jsonFlag {
			filteredConfMap := map[string]string{}
			for _, arg := range args {
				filteredConfMap[arg], _ = configuration.GetScopedConfigValue(conf, arg)
			}

			printer.JSON(filteredConfMap)
			return
		}

		var rows [][]string
		for _, arg := range args {
			value, scope := configuration.GetScopedConfigValue(conf, arg)
			rows = append(rows, []string{arg, value, scope})
		}

		printer.Table([]string{"name", "value", "scope"}, rows)
	},
}

var configureSetCmd = &cobra.Command{
	Use:   "set [options]",
	Short: "Set the value of one or more options in the config file",
	Long: `Set the value of one or more options in the config file.

Ex: set the options "key" and "otherkey":
doppler configure set key=123 otherkey=456`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("requires at least 1 arg(s), only received 0")
		}

		if !strings.Contains(args[0], "=") {
			if len(args) == 2 {
				if configuration.IsValidConfigOption(args[0]) {
					return nil
				}
				return errors.New("invalid option " + args[0])
			}

			return errors.New("too many arguments. To set multiple options, use the format option=value")
		}

		for _, arg := range args {
			option := strings.Split(arg, "=")
			if len(option) < 2 || !configuration.IsValidConfigOption(option[0]) {
				return errors.New("invalid option " + option[0])
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		silent := utils.GetBoolFlag(cmd, "silent")
		scope := cmd.Flag("scope").Value.String()
		jsonFlag := utils.OutputJSON

		if !strings.Contains(args[0], "=") {
			configuration.Set(scope, map[string]string{args[0]: args[1]})
		} else {
			options := map[string]string{}
			for _, option := range args {
				arr := strings.Split(option, "=")
				options[arr[0]] = arr[1]
			}
			configuration.Set(scope, options)
		}

		if !silent {
			printer.ScopedConfig(configuration.Get(scope), jsonFlag)
		}
	},
}

var configureUnsetCmd = &cobra.Command{
	Use:   "unset [options]",
	Short: "Unset the value of one or more options in the config file",
	Long: `Unset the value of one or more options in the config file.

Ex: unset the options "key" and "otherkey":
doppler configure unset key otherkey`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("requires at least 1 arg(s), only received 0")
		}

		for _, arg := range args {
			if !configuration.IsValidConfigOption(arg) {
				return errors.New("invalid option " + arg)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		silent := utils.GetBoolFlag(cmd, "silent")
		jsonFlag := utils.OutputJSON

		scope := cmd.Flag("scope").Value.String()
		configuration.Unset(scope, args)

		if !silent {
			printer.ScopedConfig(configuration.Get(scope), jsonFlag)
		}
	},
}

func init() {
	configureCmd.AddCommand(configureDebugCmd)

	configureGetCmd.Flags().Bool("plain", false, "print values without formatting. values will be printed in the same order as specified")
	configureCmd.AddCommand(configureGetCmd)

	configureSetCmd.Flags().Bool("silent", false, "don't output the new config")
	configureCmd.AddCommand(configureSetCmd)

	configureUnsetCmd.Flags().Bool("silent", false, "don't output the new config")
	configureCmd.AddCommand(configureUnsetCmd)

	configureCmd.Flags().Bool("all", false, "print all saved options")
	rootCmd.AddCommand(configureCmd)
}

func printScopedConfigArgs(conf models.ScopedOptions, args []string) {
	var rows [][]string
	for _, arg := range args {
		if configuration.IsValidConfigOption(arg) {
			value, scope := configuration.GetScopedConfigValue(conf, arg)
			rows = append(rows, []string{arg, value, scope})
		}
	}

	printer.Table([]string{"name", "value", "scope"}, rows)
}
