// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEffectiveAnnotationValue(t *testing.T) {
	for _, tt := range []struct {
		desc     string
		expected string
		pod      corev1.Pod
		ns       corev1.Namespace
	}{
		{
			"pod-true-overrides-ns",
			"true",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "true",
					},
				},
			},
			corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "false",
					},
				},
			},
		},

		{
			"ns-has-concrete-instance",
			"some-instance",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "true",
					},
				},
			},
			corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "some-instance",
					},
				},
			},
		},

		{
			"pod-has-concrete-instance",
			"some-instance-from-pod",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "some-instance-from-pod",
					},
				},
			},
			corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "some-instance",
					},
				},
			},
		},

		{
			"pod-has-explicit-false",
			"false",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "false",
					},
				},
			},
			corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "some-instance",
					},
				},
			},
		},

		{
			"pod-has-no-annotations",
			"some-instance",
			corev1.Pod{},
			corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "some-instance",
					},
				},
			},
		},

		{
			"ns-has-no-annotations",
			"true",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotationInjectJava: "true",
					},
				},
			},
			corev1.Namespace{},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// test
			annValue := annotationValue(tt.ns.ObjectMeta, tt.pod.ObjectMeta, annotationInjectJava)

			// verify
			assert.Equal(t, tt.expected, annValue)
		})
	}
}
