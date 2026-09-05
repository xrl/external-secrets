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

package onepasswordsdk

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/1password/onepassword-sdk-go"
	"github.com/spf13/pflag"

	"github.com/external-secrets/external-secrets/runtime/feature"
)

type sdkClientRuntime interface {
	NewClient(context.Context, ...onepassword.ClientOption) (*onepassword.Client, error)
}

type sdkRuntime struct {
	// Flags are fixed before Prepare and before any reconciler is started.
	directory string
	mu        sync.RWMutex
	owner     sdkClientRuntime
}

// The controller owner lives until process exit: manager shutdown can time out
// while reconcilers still hold clients. Never close it from SecretsClient.Close.
var controllerRuntime sdkRuntime

func init() {
	flags := pflag.NewFlagSet("onepassword-sdk", pflag.ExitOnError)
	flags.StringVar(
		&controllerRuntime.directory,
		"onepassword-sdk-cache-dir",
		"",
		"Trusted read-only 1Password SDK compilation cache; requires a cache hit at startup. Empty retains default SDK behavior.",
	)
	feature.Register(feature.Feature{Flags: flags, Prepare: controllerRuntime.prepare})
}

func (r *sdkRuntime) prepare(ctx context.Context) error {
	if r.directory == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.owner != nil {
		return nil
	}
	owner, err := onepassword.NewRuntime(onepassword.WithCompilationCache(r.directory, onepassword.CompilationCacheRequireHit))
	if err != nil {
		return err
	}
	if err := owner.Prepare(ctx); err != nil {
		return fmt.Errorf("prepare 1Password SDK require-hit cache: %w", errors.Join(err, owner.Close(context.Background())))
	}
	r.owner = owner
	return nil
}

func (r *sdkRuntime) newClient(ctx context.Context, options ...onepassword.ClientOption) (*onepassword.Client, error) {
	if r == nil || r.directory == "" {
		return onepassword.NewClient(ctx, options...)
	}
	r.mu.RLock()
	owner := r.owner
	r.mu.RUnlock()
	if owner == nil {
		return nil, errors.New("1Password SDK cache is configured but runtime is not prepared")
	}
	return owner.NewClient(ctx, options...)
}

// PrepareCompilationCache loads the same original SDK core and configuration as
// the controller, without creating clients, reading credentials or using Kubernetes.
// The standalone caller owns the runtime and always closes it before returning.
func PrepareCompilationCache(ctx context.Context, directory string, mode onepassword.CompilationCacheMode) (err error) {
	owner, err := onepassword.NewRuntime(onepassword.WithCompilationCache(directory, mode))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owner.Close(context.Background())) }()
	return owner.Prepare(ctx)
}
