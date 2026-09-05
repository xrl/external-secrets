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
	"os"
	"path/filepath"
	"testing"

	"github.com/1password/onepassword-sdk-go"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	esmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
)

type recordingRuntime struct {
	calls  int
	client *onepassword.Client
	err    error
}

func (r *recordingRuntime) NewClient(context.Context, ...onepassword.ClientOption) (*onepassword.Client, error) {
	r.calls++
	return r.client, r.err
}

func TestProviderRuntimeRouting(t *testing.T) {
	api := &fakeClient{resolveResult: "test-value", listAllResult: []onepassword.VaultOverview{{ID: "vault-id", Title: "vault"}}}
	owner := &recordingRuntime{client: &onepassword.Client{SecretsAPI: api, VaultsAPI: api}}
	state := &sdkRuntime{directory: "configured", owner: owner}
	p := NewProvider().(*Provider)
	require.Same(t, &controllerRuntime, p.runtime)
	p.runtime = state
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-token", Namespace: "test"},
		Data:       map[string][]byte{"token": []byte("synthetic-token")},
	}).Build()
	store := &esv1.SecretStore{
		TypeMeta:   metav1.TypeMeta{Kind: "SecretStore"},
		ObjectMeta: metav1.ObjectMeta{Name: "store", Namespace: "test", ResourceVersion: "1"},
		Spec: esv1.SecretStoreSpec{Provider: &esv1.SecretStoreProvider{OnePasswordSDK: &esv1.OnePasswordSDKProvider{
			Vault: "vault",
			Auth:  &esv1.OnePasswordSDKAuth{ServiceAccountSecretRef: esmeta.SecretKeySelector{Name: "test-token", Key: "token"}},
		}}},
	}
	// Store reconciliation and ordinary reconciliation both enter Provider.NewClient.
	_, err := p.ValidateStore(store)
	require.NoError(t, err)
	validationClient, err := p.NewClient(t.Context(), store, kube, "test")
	require.NoError(t, err)
	result, err := validationClient.Validate()
	require.NoError(t, err)
	require.Equal(t, esv1.ValidationResultReady, result)
	require.NoError(t, validationClient.Close(t.Context()))
	normalClient, err := p.NewClient(t.Context(), store, kube, "test")
	require.NoError(t, err)
	require.Same(t, validationClient, normalClient)
	value, err := normalClient.GetSecret(t.Context(), esv1.ExternalSecretDataRemoteRef{Key: "item/field"})
	require.NoError(t, err)
	require.Equal(t, "test-value", string(value))
	require.Equal(t, 1, owner.calls)
	store.ResourceVersion = "2"
	_, err = p.NewClient(t.Context(), store, kube, "test")
	require.NoError(t, err)
	require.Equal(t, 2, owner.calls, "new store version must use the same owner")
	owner.err = errors.New("owned client failed")
	store.ResourceVersion = "3"
	_, err = p.NewClient(t.Context(), store, kube, "test")
	require.ErrorIs(t, err, owner.err)
	state.owner = nil
	_, err = p.NewClient(t.Context(), store, kube, "test")
	require.ErrorContains(t, err, "not prepared")
}

func TestSDKRuntimePrepare(t *testing.T) {
	disabled := &sdkRuntime{}
	require.NoError(t, disabled.prepare(t.Context()))
	require.Nil(t, disabled.owner)
	// Invalid options are checked without loading/authenticating the legacy core.
	_, err := disabled.newClient(t.Context(), func(*onepassword.Client) error { return errors.New("legacy option") })
	require.ErrorContains(t, err, "legacy option")
	directory := filepath.Join(t.TempDir(), "cache")
	state := &sdkRuntime{directory: directory}
	_, err = state.newClient(t.Context())
	require.ErrorContains(t, err, "not prepared")
	require.Error(t, state.prepare(t.Context()))
	require.Nil(t, state.owner)
	_, err = os.Stat(directory)
	require.True(t, os.IsNotExist(err), "strict preparation must not create a directory")
	require.NoError(t, PrepareCompilationCache(t.Context(), directory, onepassword.CompilationCacheReadWrite))
	require.NoError(t, state.prepare(t.Context()))
	owner := state.owner
	require.NoError(t, state.prepare(t.Context()))
	require.Same(t, owner, state.owner)
	// Test owns this state; production intentionally keeps its owner until exit.
	require.NoError(t, owner.(*onepassword.Runtime).Close(context.Background()))
	_, err = state.newClient(t.Context())
	require.ErrorIs(t, err, onepassword.ErrRuntimeClosed)
}
