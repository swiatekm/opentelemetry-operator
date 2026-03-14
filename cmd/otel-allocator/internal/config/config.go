// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install"
	"github.com/spf13/pflag"
	yamlv2 "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	DefaultListenAddr                                  = ":8080"
	DefaultHttpsListenAddr                             = ":8443"
	DefaultResyncTime                                  = 5 * time.Minute
	DefaultConfigFilePath               string         = "/conf/targetallocator.yaml"
	DefaultCRScrapeInterval             model.Duration = model.Duration(time.Second * 30)
	DefaultAllocationStrategy                          = "consistent-hashing"
	DefaultFilterStrategy                              = "relabel-config"
	DefaultCollectorNotReadyGracePeriod                = 30 * time.Second
)

var DefaultKubeConfigFilePath = filepath.Join(homedir.HomeDir(), ".kube", "config")

var defaultScrapeProtocolsCR = []monitoringv1.ScrapeProtocol{
	monitoringv1.OpenMetricsText1_0_0,
	monitoringv1.OpenMetricsText0_0_1,
	monitoringv1.PrometheusText1_0_0,
	monitoringv1.PrometheusText0_0_4,
}

// logger which discards all messages written to it. Replace this with slog.DiscardHandler after we require Go 1.24.
var NopLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(math.MaxInt)}))

type Config struct {
	ListenAddr                   string                `yaml:"listen_addr,omitempty"`
	KubeConfigFilePath           string                `yaml:"kube_config_file_path,omitempty"`
	ClusterConfig                *rest.Config          `yaml:"-"`
	RootLogger                   logr.Logger           `yaml:"-"`
	CollectorSelector            *metav1.LabelSelector `yaml:"collector_selector,omitempty"`
	CollectorNamespace           string                `yaml:"collector_namespace,omitempty"`
	PromConfig                   *promconfig.Config    `yaml:"config"`
	AllocationStrategy           string                `yaml:"allocation_strategy,omitempty"`
	AllocationFallbackStrategy   string                `yaml:"allocation_fallback_strategy,omitempty"`
	FilterStrategy               string                `yaml:"filter_strategy,omitempty"`
	PrometheusCR                 PrometheusCRConfig    `yaml:"prometheus_cr,omitempty"`
	HTTPS                        HTTPSServerConfig     `yaml:"https,omitempty"`
	CollectorNotReadyGracePeriod time.Duration         `yaml:"collector_not_ready_grace_period,omitempty"`
}

type PrometheusCRConfig struct {
	Enabled                         bool                          `yaml:"enabled,omitempty"`
	AllowNamespaces                 []string                      `yaml:"allow_namespaces,omitempty"`
	DenyNamespaces                  []string                      `yaml:"deny_namespaces,omitempty"`
	PodMonitorSelector              *metav1.LabelSelector         `yaml:"pod_monitor_selector,omitempty"`
	PodMonitorNamespaceSelector     *metav1.LabelSelector         `yaml:"pod_monitor_namespace_selector,omitempty"`
	ServiceMonitorSelector          *metav1.LabelSelector         `yaml:"service_monitor_selector,omitempty"`
	ServiceMonitorNamespaceSelector *metav1.LabelSelector         `yaml:"service_monitor_namespace_selector,omitempty"`
	ScrapeConfigSelector            *metav1.LabelSelector         `yaml:"scrape_config_selector,omitempty"`
	ScrapeConfigNamespaceSelector   *metav1.LabelSelector         `yaml:"scrape_config_namespace_selector,omitempty"`
	ProbeSelector                   *metav1.LabelSelector         `yaml:"probe_selector,omitempty"`
	ProbeNamespaceSelector          *metav1.LabelSelector         `yaml:"probe_namespace_selector,omitempty"`
	ScrapeInterval                  model.Duration                `yaml:"scrape_interval,omitempty"`
	EvaluationInterval              model.Duration                `yaml:"evaluation_interval,omitempty"`
	ScrapeProtocols                 []monitoringv1.ScrapeProtocol `yaml:"scrape_protocols,omitempty"`
	ScrapeClasses                   []monitoringv1.ScrapeClass    `yaml:"scrape_classes,omitempty"`
}

type HTTPSServerConfig struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	ListenAddr      string `yaml:"listen_addr,omitempty"`
	CAFilePath      string `yaml:"ca_file_path,omitempty"`
	TLSCertFilePath string `yaml:"tls_cert_file_path,omitempty"`
	TLSKeyFilePath  string `yaml:"tls_key_file_path,omitempty"`
}

// StringToModelOrTimeDurationHookFunc returns a DecodeHookFuncType
// that converts string to time.Duration, which can also be used
// as model.Duration.
func StringToModelOrTimeDurationHookFunc() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		if t != reflect.TypeFor[model.Duration]() && t != reflect.TypeFor[time.Duration]() {
			return data, nil
		}

		return time.ParseDuration(data.(string))
	}
}

// MapToPromConfig returns a DecodeHookFuncType that provides a mechanism
// for decoding promconfig.Config involving its own unmarshal logic.
func MapToPromConfig() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f.Kind() != reflect.Map {
			return data, nil
		}

		if t != reflect.TypeFor[*promconfig.Config]() {
			return data, nil
		}

		mb, err := yamlv2.Marshal(data)
		if err != nil {
			return nil, err
		}

		pConfig := &promconfig.Config{}
		err = yamlv2.Unmarshal(mb, pConfig)
		if err != nil {
			return nil, err
		}
		return pConfig, nil
	}
}

// MapToLabelSelector returns a DecodeHookFuncType that
// provides a mechanism for decoding both matchLabels and matchExpressions from camelcase to lowercase
// because we use yaml unmarshaling that supports lowercase field names if no `yaml` tag is defined
// and metav1.LabelSelector uses `json` tags.
// If both the camelcase and lowercase version is present, then the camelcase version takes precedence.
func MapToLabelSelector() mapstructure.DecodeHookFuncType {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		if f.Kind() != reflect.Map {
			return data, nil
		}

		if t != reflect.TypeFor[*metav1.LabelSelector]() {
			return data, nil
		}

		result := &metav1.LabelSelector{}
		fMap := data.(map[string]any)
		if matchLabels, ok := fMap["matchLabels"]; ok {
			fMap["matchlabels"] = matchLabels
			delete(fMap, "matchLabels")
		}
		if matchExpressions, ok := fMap["matchExpressions"]; ok {
			fMap["matchexpressions"] = matchExpressions
			delete(fMap, "matchExpressions")
		}

		b, err := yamlv2.Marshal(fMap)
		if err != nil {
			return nil, err
		}

		err = yamlv2.Unmarshal(b, result)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
}

// LoadFromFile loads a YAML config file into the koanf instance.
func LoadFromFile(k *koanf.Koanf, configFile string) error {
	return k.Load(file.Provider(configFile), yaml.Parser())
}

// envToConfigKeyMap maps environment variable names to their corresponding koanf config key paths.
var envToConfigKeyMap = map[string]string{
	"OTELCOL_NAMESPACE": "collector_namespace",
}

// LoadFromEnv loads configuration from environment variables into the koanf instance.
func LoadFromEnv(k *koanf.Koanf) error {
	return k.Load(env.ProviderWithValue("", ".", func(key, value string) (string, any) {
		configKey, ok := envToConfigKeyMap[key]
		if !ok {
			return "", nil
		}
		return configKey, value
	}), nil)
}

// LoadFromCLI loads changed CLI flag values into the koanf instance.
func LoadFromCLI(k *koanf.Koanf, flagSet *pflag.FlagSet) error {
	return k.Load(posflag.ProviderWithFlag(flagSet, ".", nil, flagToConfigKey(flagSet)), nil)
}

// Unmarshal decodes the koanf contents into the cfg argument, using mapstructure
// with the following notable behaviors:
//   - Decodes time.Duration from strings (see StringToModelOrTimeDurationHookFunc).
//   - Allows custom unmarshaling for promconfig.Config (see MapToPromConfig).
//   - Allows custom unmarshaling for metav1.LabelSelector using both camelcase and
//     lowercase field names (see MapToLabelSelector).
func Unmarshal(k *koanf.Koanf, cfg *Config) error {
	return k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		Tag: "yaml",
		DecoderConfig: &mapstructure.DecoderConfig{
			TagName: "yaml",
			Result:  cfg,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				StringToModelOrTimeDurationHookFunc(),
				MapToPromConfig(),
				MapToLabelSelector(),
			),
		},
	})
}

func CreateDefaultConfig() Config {
	return Config{
		ListenAddr:         DefaultListenAddr,
		KubeConfigFilePath: DefaultKubeConfigFilePath,
		HTTPS: HTTPSServerConfig{
			ListenAddr: DefaultHttpsListenAddr,
		},
		AllocationStrategy:         DefaultAllocationStrategy,
		AllocationFallbackStrategy: "",
		FilterStrategy:             DefaultFilterStrategy,
		PrometheusCR: PrometheusCRConfig{
			ScrapeInterval:                  DefaultCRScrapeInterval,
			ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
			PodMonitorNamespaceSelector:     &metav1.LabelSelector{},
			ScrapeConfigNamespaceSelector:   &metav1.LabelSelector{},
			ProbeNamespaceSelector:          &metav1.LabelSelector{},
			ScrapeProtocols:                 defaultScrapeProtocolsCR,
		},
		CollectorNotReadyGracePeriod: DefaultCollectorNotReadyGracePeriod,
	}
}

func Load(args []string) (*Config, error) {
	flagSet := getFlagSet(pflag.ExitOnError)
	if err := flagSet.Parse(args); err != nil {
		return nil, err
	}

	k := koanf.New(".")

	// Load sources in priority order: file < env < CLI
	configFilePath, err := getConfigFilePath(flagSet)
	if err != nil {
		return nil, err
	}
	if err := LoadFromFile(k, configFilePath); err != nil {
		return nil, err
	}
	if err := LoadFromEnv(k); err != nil {
		return nil, err
	}
	if err := LoadFromCLI(k, flagSet); err != nil {
		return nil, err
	}

	// Unmarshal the merged config into the struct
	config := CreateDefaultConfig()
	if err := Unmarshal(k, &config); err != nil {
		return nil, err
	}

	// Set up logger from CLI flags
	config.RootLogger = zap.New(zap.UseFlagOptions(&zapCmdLineOpts))
	klog.SetLogger(config.RootLogger)
	ctrl.SetLogger(config.RootLogger)

	// Build cluster config
	clusterConfig, err := clientcmd.BuildConfigFromFlags("", config.KubeConfigFilePath)
	if err != nil {
		pathError := &fs.PathError{}
		if ok := errors.As(err, &pathError); !ok {
			return nil, err
		}
		clusterConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		config.KubeConfigFilePath = ""
	}
	config.ClusterConfig = clusterConfig

	return &config, nil
}

// ValidateConfig validates the cli and file configs together.
func ValidateConfig(config *Config) error {
	scrapeConfigsPresent := (config.PromConfig != nil && len(config.PromConfig.ScrapeConfigs) > 0)
	if !config.PrometheusCR.Enabled && !scrapeConfigsPresent {
		return errors.New("at least one scrape config must be defined, or Prometheus CR watching must be enabled")
	}
	if config.CollectorNamespace == "" {
		return errors.New("collector namespace must be set")
	}
	if len(config.PrometheusCR.AllowNamespaces) != 0 && len(config.PrometheusCR.DenyNamespaces) != 0 {
		return errors.New("only one of allowNamespaces or denyNamespaces can be set")
	}
	return nil
}

func (c HTTPSServerConfig) NewTLSConfig(logger logr.Logger) (*tls.Config, *certwatcher.CertWatcher, error) {
	// Create certwatcher for server certificate/key reloading
	certWatcher, err := certwatcher.New(c.TLSCertFilePath, c.TLSKeyFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cert watcher: %w", err)
	}

	// Create CA reloader for client CA certificate reloading
	caReloader, err := NewCAReloader(c.CAFilePath, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA reloader: %w", err)
	}

	// Register callback to reload CA when server cert changes
	// Since Kubernetes updates secrets atomically, the CA will be updated at the same time
	certWatcher.RegisterCallback(func(tls.Certificate) {
		if reloadErr := caReloader.Reload(); reloadErr != nil {
			logger.Error(reloadErr, "Failed to reload CA via callback")
		}
	})

	tlsConfig := &tls.Config{
		GetCertificate: certWatcher.GetCertificate,
		// Request client certificate but don't verify automatically
		// We'll do custom verification in VerifyConnection with the dynamic CA pool
		ClientAuth: tls.RequestClientCert,
		MinVersion: tls.VersionTLS12,
		// Use VerifyConnection for dynamic CA pool access
		// This allows the CA pool to be reloaded at runtime
		VerifyConnection: func(cs tls.ConnectionState) error {
			// Require client certificate
			if len(cs.PeerCertificates) == 0 {
				return errors.New("no client certificate provided")
			}

			// Verify using current CA pool (which can be reloaded)
			opts := x509.VerifyOptions{
				Roots:         caReloader.GetClientCAs(),
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			}

			// Add intermediate certificates to the pool
			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}

			// Verify only the leaf certificate
			if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
				return fmt.Errorf("client certificate verification failed: %w", err)
			}
			return nil
		},
	}

	return tlsConfig, certWatcher, nil
}

// GetAllowDenyLists returns the allow and deny lists as maps. If the allow list is empty, it defaults to all namespaces.
// If the deny list is empty, it defaults to an empty map.
func (c PrometheusCRConfig) GetAllowDenyLists() (allowList, denyList map[string]struct{}) {
	allowList = map[string]struct{}{}
	if len(c.AllowNamespaces) != 0 {
		for _, ns := range c.AllowNamespaces {
			allowList[ns] = struct{}{}
		}
	} else {
		allowList = map[string]struct{}{v1.NamespaceAll: {}}
	}

	denyList = map[string]struct{}{}
	if len(c.DenyNamespaces) != 0 {
		for _, ns := range c.DenyNamespaces {
			denyList[ns] = struct{}{}
		}
	}

	return allowList, denyList
}
