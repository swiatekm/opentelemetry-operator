// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestGetFlagSet(t *testing.T) {
	fs := getFlagSet(pflag.ExitOnError)

	// Check if each flag exists
	assert.NotNil(t, fs.Lookup(configFilePathFlagName), "Flag %s not found", configFilePathFlagName)
	assert.NotNil(t, fs.Lookup(listenAddrFlagName), "Flag %s not found", listenAddrFlagName)
	assert.NotNil(t, fs.Lookup(prometheusCREnabledFlagName), "Flag %s not found", prometheusCREnabledFlagName)
	assert.NotNil(t, fs.Lookup(kubeConfigPathFlagName), "Flag %s not found", kubeConfigPathFlagName)
}

func TestFlagToConfigKey(t *testing.T) {
	tests := []struct {
		name        string
		flagArgs    []string
		flagName    string
		expectedKey string
		expectSkip  bool
	}{
		{
			name:        "listen addr maps to listen_addr",
			flagArgs:    []string{"--" + listenAddrFlagName, ":8081"},
			flagName:    listenAddrFlagName,
			expectedKey: "listen_addr",
		},
		{
			name:        "kubeconfig maps to kube_config_file_path",
			flagArgs:    []string{"--" + kubeConfigPathFlagName, "/some/path"},
			flagName:    kubeConfigPathFlagName,
			expectedKey: "kube_config_file_path",
		},
		{
			name:        "prometheus CR enabled maps to prometheus_cr.enabled",
			flagArgs:    []string{"--" + prometheusCREnabledFlagName, "true"},
			flagName:    prometheusCREnabledFlagName,
			expectedKey: "prometheus_cr.enabled",
		},
		{
			name:        "https enabled maps to https.enabled",
			flagArgs:    []string{"--" + httpsEnabledFlagName, "true"},
			flagName:    httpsEnabledFlagName,
			expectedKey: "https.enabled",
		},
		{
			name:        "https key file maps to https.tls_key_file_path",
			flagArgs:    []string{"--" + httpsTLSKeyFilePathFlagName, "/path/to/tls.key"},
			flagName:    httpsTLSKeyFilePathFlagName,
			expectedKey: "https.tls_key_file_path",
		},
		{
			name:       "unchanged flag is skipped",
			flagArgs:   []string{},
			flagName:   listenAddrFlagName,
			expectSkip: true,
		},
		{
			name:       "config-file flag is not mapped",
			flagArgs:   []string{"--" + configFilePathFlagName, "/some/path"},
			flagName:   configFilePathFlagName,
			expectSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := getFlagSet(pflag.ContinueOnError)
			err := fs.Parse(tt.flagArgs)
			assert.NoError(t, err)

			f := fs.Lookup(tt.flagName)
			assert.NotNil(t, f)

			key, _ := flagToConfigKey(fs)(f)
			if tt.expectSkip {
				assert.Empty(t, key, "expected flag to be skipped")
			} else {
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestGetConfigFilePath(t *testing.T) {
	fs := getFlagSet(pflag.ContinueOnError)
	err := fs.Parse([]string{"--" + configFilePathFlagName, "/path/to/config"})
	assert.NoError(t, err)

	got, err := getConfigFilePath(fs)
	assert.NoError(t, err)
	assert.Equal(t, "/path/to/config", got)
}
