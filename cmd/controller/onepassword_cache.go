//go:build onepasswordsdk || all_providers

/*
Copyright © The ESO Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"runtime"

	"github.com/1password/onepassword-sdk-go"
	"github.com/spf13/cobra"
	"golang.org/x/sys/cpu"

	"github.com/external-secrets/external-secrets/providers/v1/onepasswordsdk"
)

func newOnePasswordCacheCommand() *cobra.Command {
	var directory string
	command := &cobra.Command{
		Use:   "onepassword-sdk-cache [prepare|check]",
		Short: "Prepare or require-hit check the trusted SDK cache without credentials or Kubernetes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var mode onepassword.CompilationCacheMode
			switch args[0] {
			case "prepare":
				mode = onepassword.CompilationCacheReadWrite
			case "check":
				mode = onepassword.CompilationCacheRequireHit
			default:
				return fmt.Errorf("expected prepare or check, got %q", args[0])
			}
			if err := onepasswordsdk.PrepareCompilationCache(cmd.Context(), directory, mode); err != nil {
				return fmt.Errorf("1Password SDK cache %s failed: %w", args[0], err)
			}
			cmd.Printf("1Password SDK cache %s succeeded: %s/%s arm64-lse=%t\n", args[0], runtime.GOOS, runtime.GOARCH, cpu.ARM64.HasATOMICS)
			return nil
		},
	}
	command.Flags().StringVar(&directory, "cache-dir", "", "Trusted executable cache directory (required)")
	return command
}

func init() { rootCmd.AddCommand(newOnePasswordCacheCommand()) }
