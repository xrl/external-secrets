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
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnePasswordCacheCommand(t *testing.T) {
	// Invalid Kubernetes config must be irrelevant to every standalone path.
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent-kubeconfig"))
	for _, args := range [][]string{{}, {"wrong"}, {"prepare"}, {"check", "--cache-dir", filepath.Join(t.TempDir(), "missing")}} {
		cmd := newOnePasswordCacheCommand()
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute())
	}
	directory := filepath.Join(t.TempDir(), "cache")
	for _, mode := range []string{"prepare", "check"} {
		cmd := newOnePasswordCacheCommand()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs([]string{mode, "--cache-dir", directory})
		require.NoError(t, cmd.Execute())
		require.Contains(t, output.String(), "cache "+mode+" succeeded")
		require.Contains(t, output.String(), "arm64-lse=")
	}
}
