// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"flag"

	"github.com/knadh/koanf/providers/posflag"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Flag names.
const (
	targetAllocatorName          = "target-allocator"
	configFilePathFlagName       = "config-file"
	listenAddrFlagName           = "listen-addr"
	prometheusCREnabledFlagName  = "enable-prometheus-cr-watcher"
	kubeConfigPathFlagName       = "kubeconfig-path"
	httpsEnabledFlagName         = "enable-https-server"
	listenAddrHttpsFlagName      = "listen-addr-https"
	httpsCAFilePathFlagName      = "https-ca-file"
	httpsTLSCertFilePathFlagName = "https-tls-cert-file"
	httpsTLSKeyFilePathFlagName  = "https-tls-key-file"
)

// flagToConfigKeyMap maps CLI flag names to their corresponding koanf config key paths.
var flagToConfigKeyMap = map[string]string{
	listenAddrFlagName:           "listen_addr",
	kubeConfigPathFlagName:       "kube_config_file_path",
	prometheusCREnabledFlagName:  "prometheus_cr.enabled",
	httpsEnabledFlagName:         "https.enabled",
	listenAddrHttpsFlagName:      "https.listen_addr",
	httpsCAFilePathFlagName:      "https.ca_file_path",
	httpsTLSCertFilePathFlagName: "https.tls_cert_file_path",
	httpsTLSKeyFilePathFlagName:  "https.tls_key_file_path",
}

// We can't bind this flag to our FlagSet, so we need to handle it separately.
var zapCmdLineOpts zap.Options

func getFlagSet(errorHandling pflag.ErrorHandling) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet(targetAllocatorName, errorHandling)
	flagSet.String(configFilePathFlagName, DefaultConfigFilePath, "The path to the config file.")
	flagSet.String(listenAddrFlagName, DefaultListenAddr, "The address where this service serves.")
	flagSet.Bool(prometheusCREnabledFlagName, false, "Enable Prometheus CRs as target sources")
	flagSet.String(kubeConfigPathFlagName, DefaultKubeConfigFilePath, "absolute path to the KubeconfigPath file")
	flagSet.Bool(httpsEnabledFlagName, false, "Enable HTTPS additional server")
	flagSet.String(listenAddrHttpsFlagName, DefaultHttpsListenAddr, "The address where this service serves over HTTPS.")
	flagSet.String(httpsCAFilePathFlagName, "", "The path to the HTTPS server TLS CA file.")
	flagSet.String(httpsTLSCertFilePathFlagName, "", "The path to the HTTPS server TLS certificate file.")
	flagSet.String(httpsTLSKeyFilePathFlagName, "", "The path to the HTTPS server TLS key file.")
	zapFlagSet := flag.NewFlagSet("", flag.ErrorHandling(errorHandling))
	zapCmdLineOpts.BindFlags(zapFlagSet)
	flagSet.AddGoFlagSet(zapFlagSet)
	return flagSet
}

func getConfigFilePath(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(configFilePathFlagName)
}

// flagToConfigKey maps a pflag.Flag to its koanf config key path and parsed value.
// Only changed flags are mapped; unchanged flags return an empty key and are skipped.
func flagToConfigKey(fs *pflag.FlagSet) func(f *pflag.Flag) (string, any) {
	return func(f *pflag.Flag) (string, any) {
		if !f.Changed {
			return "", nil
		}
		key, ok := flagToConfigKeyMap[f.Name]
		if !ok {
			return "", nil
		}
		return key, posflag.FlagVal(fs, f)
	}
}
