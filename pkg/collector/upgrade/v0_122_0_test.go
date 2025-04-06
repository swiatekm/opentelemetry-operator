// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upgrade_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/open-telemetry/opentelemetry-operator/pkg/collector/upgrade"
)

func Test0_122_0Upgrade(t *testing.T) {
	defaultCollector := v1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-my-instance",
			Namespace: "somewhere",
		},
		Status: v1beta1.OpenTelemetryCollectorStatus{
			Version: "0.121.0",
		},
		Spec: v1beta1.OpenTelemetryCollectorSpec{
			OpenTelemetryCommonFields: v1beta1.OpenTelemetryCommonFields{},
			Config:                    v1beta1.Config{},
		},
	}

	defaultCollectorWithConfig := defaultCollector.DeepCopy()

	defaultCollectorWithConfig.Spec.Config.Service.Telemetry = &v1beta1.AnyConfig{
		Object: map[string]interface{}{
			"metrics": map[string]interface{}{
				"level":   "basic",
				"address": "1.2.3.4:8888",
			},
		},
	}

	tt := []struct {
		name     string
		input    v1beta1.OpenTelemetryCollector
		expected v1beta1.OpenTelemetryCollector
	}{
		{
			name:     "no metrics address set",
			input:    defaultCollector,
			expected: defaultCollector,
		},
		{
			name:  "telemetry settings do not exist",
			input: *defaultCollectorWithConfig.DeepCopy(),
			expected: func() v1beta1.OpenTelemetryCollector {
				col := defaultCollector.DeepCopy()
				col.Spec.Config.Service.Telemetry = &v1beta1.AnyConfig{
					Object: map[string]interface{}{
						"metrics": map[string]interface{}{
							"level": "basic",
							"readers": []any{
								map[string]any{
									"pull": map[string]interface{}{
										"exporter": map[string]interface{}{
											"prometheus": map[string]interface{}{
												"host": "1.2.3.4",
												"port": 8888.0,
											},
											"AdditionalProperties": nil,
										},
									},
								},
							},
						},
					},
				}
				return *col
			}(),
		},
	}

	versionUpgrade := &upgrade.VersionUpgrade{
		Log:      logger,
		Version:  makeVersion("0.122.0"),
		Client:   k8sClient,
		Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			col, err := versionUpgrade.ManagedInstance(context.Background(), tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected.Spec.Config.Service.Telemetry, col.Spec.Config.Service.Telemetry)
		})
	}
}
