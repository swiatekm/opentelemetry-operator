// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e_collector_metrics

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"testing"
	"text/template"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/open-telemetry/opentelemetry-operator/internal/testing/e2e"
)

// These tests validate that the target allocator labels targets correctly, end to
// end. A collector (TA + prometheus receiver) is deployed through the operator,
// scrapes a sample app, and exports OTLP into a single prometheus-operator-managed
// Prometheus running with translation_strategy: NoTranslation (so target labels cross
// the OTLP boundary byte-for-byte). We assert purely on target labels:
//
//   - ServiceMonitor (prometheusCR) path: a live differential. The SAME Prometheus
//     both receives the TA pipeline (pipeline=ta) and natively scrapes the same pod
//     via prometheus-operator (pipeline=oracle). The two must carry identical target
//     labels — the allocator must label a ServiceMonitor target exactly as
//     prometheus-operator does.
//   - raw scrape_configs path: the static target carries exactly job/instance and no
//     service-discovery labels.

const (
	sampleApp = "sample-app"          // the scraped workload (Deployment + Service)
	promSvc   = "prometheus-operated" // headless Service prometheus-operator creates for the Prometheus pods
	promPort  = 9090

	nsPrefix   = "e2e-metrics"       // prefix for the random per-test namespace
	oracleName = "oracle"            // name of the oracle Prometheus CR and its ServiceAccount
	oracleSTS  = "prometheus-oracle" // StatefulSet prometheus-operator derives from the oracle Prometheus

	pipelineLabel = "pipeline" // label that distinguishes the pipelines in one Prometheus
	pipelineTA    = "ta"       // TA pipeline: collector(TA) -> OTLP -> Prometheus
	pipelineProm  = "oracle"   // oracle pipeline: prometheus-operator native scrape

	smTA     = "sm-ta"     // ServiceMonitor the collector's target allocator selects
	smOracle = "sm-oracle" // ServiceMonitor the oracle Prometheus scrapes natively
)

//go:embed testdata
var manifestFS embed.FS

// render reads an embedded manifest template and executes it with data. The templates
// are static and validated by the tests, so a parse/execute failure is a programming
// error and panics.
func render(name string, data any) string {
	b, err := manifestFS.ReadFile("testdata/" + name)
	if err != nil {
		panic(err)
	}
	var buf bytes.Buffer
	if err := template.Must(template.New(name).Parse(string(b))).Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}

func sampleAppManifests() string { return render("sample-app.yaml", nil) }

// oraclePrometheus renders the oracle Prometheus. selectGroup, when non-empty, is the
// ServiceMonitor group it scrapes natively; empty makes it a pure OTLP sink.
func oraclePrometheus(selectGroup string) string {
	return render("oracle-prometheus.yaml", map[string]string{"Name": oracleName, "SelectGroup": selectGroup})
}

func serviceMonitor(name, group, pipeline string) string {
	return render("service-monitor.yaml", map[string]string{"Name": name, "Group": group, "Pipeline": pipeline})
}

func smCollectorCR(name, group string) string {
	return render("collector-sm.yaml", map[string]string{"Name": name, "Group": group})
}

func rawCollectorCR(name, app string) string {
	return render("collector-raw.yaml", map[string]string{"Name": name, "App": app})
}

var testenv env.Environment

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to parse e2e flags: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	os.Exit(testenv.Run(m))
}

type nsKey struct{}

func ns(ctx context.Context) string { return ctx.Value(nsKey{}).(string) }

// setup ensures prometheus-operator is installed, then deploys the sample app, the
// oracle Prometheus (+ RBAC), any extra manifests (e.g. ServiceMonitors), the
// collector CR (+ RBAC), waits for readiness, and stashes the namespace in the
// context. oracleGroup selects which ServiceMonitors the oracle Prometheus scrapes
// natively (empty = none; it is then just an OTLP sink).
func setup(crName, collectorCR, oracleGroup string, extra ...string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		e2e.EnsurePrometheusOperator(ctx, t, cfg)

		namespace := envconf.RandomName(nsPrefix, 16)
		e2e.CreateNamespace(ctx, t, cfg, namespace)
		t.Cleanup(func() { e2e.DeleteNamespace(context.WithoutCancel(ctx), t, cfg, namespace) })

		e2e.Apply(ctx, t, cfg, namespace, sampleAppManifests())
		e2e.Apply(ctx, t, cfg, namespace, oraclePrometheus(oracleGroup))
		e2e.BindTargetAllocatorClusterRole(ctx, t, cfg, namespace, oracleName)
		for _, m := range extra {
			e2e.Apply(ctx, t, cfg, namespace, m)
		}
		e2e.Apply(ctx, t, cfg, namespace, collectorCR)
		// The operator names the target allocator ServiceAccount <cr>-targetallocator.
		e2e.BindTargetAllocatorClusterRole(ctx, t, cfg, namespace, crName+"-targetallocator")

		e2e.WaitForDeployment(ctx, t, cfg, namespace, sampleApp, 2*time.Minute)
		// prometheus-operator reconciles the Prometheus CR into the prometheus-oracle
		// StatefulSet; the operator reconciles the collector CR into <name>-collector.
		e2e.WaitForStatefulSet(ctx, t, cfg, namespace, oracleSTS, 1, 3*time.Minute)
		e2e.WaitForStatefulSet(ctx, t, cfg, namespace, crName+"-collector", 1, 5*time.Minute)
		return context.WithValue(ctx, nsKey{}, namespace)
	}
}

// TestServiceMonitorDifferential is the headline test: the target allocator must
// label a ServiceMonitor target exactly as prometheus-operator does. Both pipelines
// scrape the same pod and write to the same Prometheus, distinguished by a `pipeline`
// label, so any divergence in target labeling fails the differential.
func TestServiceMonitorDifferential(t *testing.T) {
	const crName = "sm"
	prom := e2e.PromTarget{Service: promSvc, Port: promPort}

	feat := features.New("ServiceMonitor targets labeled identically to prometheus-operator").
		Setup(setup(crName, smCollectorCR(crName, pipelineTA), pipelineProm,
			serviceMonitor(smTA, pipelineTA, pipelineTA),
			serviceMonitor(smOracle, pipelineProm, pipelineProm))).
		Assess("the allocator path really went through ServiceMonitor relabeling", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Guards the differential against a vacuous pass: the TA series must carry
			// the prometheus-operator relabeling output, not be empty/trivial.
			e2e.EventuallyPromQL(ctx, t, cfg, ns(ctx), prom, fmt.Sprintf(`up{%s=%q}`, pipelineLabel, pipelineTA),
				e2e.HasSeries(e2e.Series{
					Labels:  map[string]string{"service": sampleApp, "namespace": ns(ctx)},
					Present: []string{"endpoint", "pod", "container"},
				}))
			return ctx
		}).
		Assess("target identity matches prometheus-operator (up)", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			e2e.SameLabelsAcross(ctx, t, cfg, ns(ctx), prom, `up`, pipelineLabel, 2)
			return ctx
		}).
		Assess("a scraped series matches prometheus-operator end-to-end (version)", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			e2e.SameLabelsAcross(ctx, t, cfg, ns(ctx), prom, `version`, pipelineLabel, 2)
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

// TestRawScrapeConfigMetrics validates the target allocator's raw scrape_configs
// path: a static_configs target is allocated, scraped, exported OTLP and arrives
// carrying exactly the scrape config's identity (job/instance) and no
// service-discovery labels.
func TestRawScrapeConfigMetrics(t *testing.T) {
	const crName = "raw"
	prom := e2e.PromTarget{Service: promSvc, Port: promPort}

	feat := features.New("raw scrape_configs carry exactly the static identity").
		Setup(setup(crName, rawCollectorCR(crName, sampleApp), "")).
		Assess("the static target carries exactly job/instance and no service-discovery labels", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			e2e.EventuallyPromQL(ctx, t, cfg, ns(ctx), prom, fmt.Sprintf(`up{job=%q}`, sampleApp),
				e2e.HasSeries(e2e.Series{
					Labels: map[string]string{"job": sampleApp, "instance": sampleApp + ":8080"},
					Exact:  true,
				}))
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
