// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"dario.cat/mergo"
	"github.com/goccy/go-json"

	otelConfig "go.opentelemetry.io/contrib/otelconf/v0.3.0"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
)

func upgrade0_122_0(_ VersionUpgrade, otelcol *v1beta1.OpenTelemetryCollector) (*v1beta1.OpenTelemetryCollector, error) {
	err := migrateTelemetrySettings(otelcol)
	if err != nil {
		return nil, err
	}
	return otelcol, nil
}

// migrateTelemetrySettings migrates telemetry settings of an otel collector from the older, deprecated format
// to the new one based on the otel SDK.
// This effectively reproduces the logic from
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.122.0/service/telemetry/config.go#L170
// See also https://github.com/open-telemetry/opentelemetry-collector/pull/11205.
func migrateTelemetrySettings(otelcol *v1beta1.OpenTelemetryCollector) error {
	telemetryConfig := otelcol.Spec.Config.Service.Telemetry
	if telemetryConfig == nil {
		return nil
	}

	var telemetry Telemetry
	jsonData, err := json.Marshal(telemetryConfig)
	if err != nil {
		return err
	}
	// Unmarshal JSON into the provided struct
	if uErr := json.Unmarshal(jsonData, &telemetry); uErr != nil {
		return uErr
	}

	if len(telemetry.Metrics.Address) == 0 {
		return nil
	}

	host, port, err := splitAddressHostPort(telemetry.Metrics.Address)
	if err != nil {
		return fmt.Errorf("unable to extract host and port from address: %w", err)
	}

	telemetry.Metrics.Readers = append(telemetry.Metrics.Readers, otelConfig.MetricReader{
		Pull: &otelConfig.PullMetricReader{
			Exporter: otelConfig.PullMetricExporter{
				Prometheus: &otelConfig.Prometheus{
					Host: &host,
					Port: &port,
				},
			},
		},
	})

	// unset the address
	telemetry.Metrics.Address = ""

	// serialize back into a map
	var telemetryConfigMap map[string]any
	jsonData, err = json.Marshal(telemetry)
	if err != nil {
		return err
	}
	if uErr := json.Unmarshal(jsonData, &telemetryConfigMap); uErr != nil {
		return uErr
	}

	// merge the map back into the collector config
	err = mergo.Merge(&telemetryConfig.Object, telemetryConfigMap, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
	if err != nil {
		return err
	}

	return nil
}

// splitAddressHostPort splits the provided address, returning the host and port. The logic is the same as
// net.SplitHostPort, but we parse the port into an int32.
func splitAddressHostPort(address string) (string, int, error) {
	// if the string contains variable references, return an error
	if strings.Contains(address, "$") {
		return "", 0, fmt.Errorf("cannot parse address containing variable references: %s", address)
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	portInt, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to extract the port from the metrics address %q: %w", address, err)
	}
	return host, portInt, nil
}

// Redeclare some structs from v1beta1 here to avoid depending on them

// MetricsConfig comes from the collector.
type MetricsConfig struct {
	// Level is the level of telemetry metrics, the possible values are:
	//  - "none" indicates that no telemetry data should be collected;
	//  - "basic" is the recommended and covers the basics of the service telemetry.
	//  - "normal" adds some other indicators on top of basic.
	//  - "detailed" adds dimensions and views to the previous levels.
	Level string `json:"level,omitempty" yaml:"level,omitempty"`

	// Address is the [address]:port that metrics exposition should be bound to.
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	otelConfig.MeterProvider `mapstructure:",squash"`
}

// Telemetry is an intermediary type that allows for easy access to the collector's telemetry settings.
type Telemetry struct {
	Metrics MetricsConfig `json:"metrics,omitempty" yaml:"metrics,omitempty"`

	// Resource specifies user-defined attributes to include with all emitted telemetry.
	// Note that some attributes are added automatically (e.g. service.version) even
	// if they are not specified here. In order to suppress such attributes the
	// attribute must be specified in this map with null YAML value (nil string pointer).
	Resource map[string]*string `json:"resource,omitempty" yaml:"resource,omitempty"`
}
