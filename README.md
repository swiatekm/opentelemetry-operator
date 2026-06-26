[![Continuous Integration][github-workflow-img]][github-workflow] [![Go Report Card][goreport-img]][goreport] [![GoDoc][godoc-img]][godoc] [![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/open-telemetry/opentelemetry-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/open-telemetry/opentelemetry-operator)

# OpenTelemetry Operator for Kubernetes

The OpenTelemetry Operator is an implementation of a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

The operator manages:

- [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)
- [auto-instrumentation](https://opentelemetry.io/docs/concepts/instrumentation/automatic/) of the workloads using OpenTelemetry instrumentation libraries

## Documentation

User documentation lives under [`docs/`](docs/README.md):

- [Getting started](docs/getting-started/README.md) — installation, upgrades, compatibility
- [Concepts](docs/concepts/README.md) — what the operator manages
- [Collector](docs/collector/README.md) — `OpenTelemetryCollector` CRD: deployment modes, sidecar injection, observability
- [Auto-instrumentation](docs/auto-instrumentation/README.md) — `Instrumentation` CRD: per-language injection, resource attributes
- [Target Allocator](docs/target-allocator/README.md) — Prometheus scrape target distribution and discovery
- [OpAMP Bridge](docs/opamp-bridge/README.md) — `OpAMPBridge` CRD
- [Use cases](docs/use-cases/README.md) — task-oriented guides (planned)
- [Reference architectures](docs/architectures/README.md) — deployment patterns (planned)
- [Security](docs/security/README.md) — RBAC, TLS, certificates (planned)
- [Troubleshooting](docs/troubleshooting/README.md) — debug tips and known issues
- [Reference](docs/reference/README.md) — API docs, CRD changelog, feature gates
- [RFCs](docs/rfcs/README.md) — design proposals
- [Official OpenTelemetry Operator page](https://opentelemetry.io/docs/platforms/kubernetes/operator/)

## Helm Charts

You can install OpenTelemetry Operator via [Helm Chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-operator) from the opentelemetry-helm-charts repository. More information is available in [here](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-operator).

## Getting started

To install the operator in an existing cluster, make sure you have [`cert-manager` installed](https://cert-manager.io/docs/installation/) and run:

```bash
kubectl apply -f https://github.com/open-telemetry/opentelemetry-operator/releases/latest/download/opentelemetry-operator.yaml
```

Once the `opentelemetry-operator` deployment is ready, create an OpenTelemetry Collector (otelcol) instance, like:

```yaml
kubectl apply -f - <<EOF
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: simplest
spec:
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
    processors:
      memory_limiter:
        check_interval: 1s
        limit_percentage: 75
        spike_limit_percentage: 15

    exporters:
      debug: {}

    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter]
          exporters: [debug]
EOF
```

**_WARNING:_** Until the OpenTelemetry Collector format is stable, changes may be required in the above example to remain
compatible with the latest version of the OpenTelemetry Collector image being referenced.

This will create an OpenTelemetry Collector instance named `simplest`, exposing a `jaeger-grpc` port to consume spans from your instrumented applications and exporting those spans via `debug`, which writes the spans to the console (`stdout`) of the OpenTelemetry Collector instance that receives the span.

The `config` node holds the `YAML` that should be passed down as-is to the underlying OpenTelemetry Collector instances. Refer to the [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector) documentation for a reference of the possible entries.

> 🚨 **NOTE:** At this point, the Operator does _not_ validate the whole contents of the configuration file: if the configuration is invalid, the instance might still be created but the underlying OpenTelemetry Collector might crash.

> 🚨 **Note:** For private GKE clusters, you will need to either add a firewall rule that allows master nodes access to port `9443/tcp` on worker nodes, or change the existing rule that allows access to port `80/tcp`, `443/tcp` and `10254/tcp` to also allow access to port `9443/tcp`. More information can be found in the [Official GCP Documentation](https://cloud.google.com/load-balancing/docs/tcp/setting-up-tcp#config-hc-firewall). See the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/private-clusters#add_firewall_rules) on adding rules and the [Kubernetes issue](https://github.com/kubernetes/kubernetes/issues/79739) for more detail.

The Operator does examine the configuration file for a few purposes:

- To discover configured receivers and their ports. If it finds receivers with ports, it creates a pair of kubernetes services, one headless, exposing those ports within the cluster. If the port is using environment variable expansion or cannot be parsed, an error will be returned. The headless service contains a `service.beta.openshift.io/serving-cert-secret-name` annotation that will cause OpenShift to create a secret containing a certificate and key. This secret can be mounted as a volume and the certificate and key used in those receivers' TLS configurations.

- To check if Collector observability is enabled (controlled by `spec.observability.metrics.enableMetrics`). In this case, a Service and ServiceMonitor/PodMonitor are created for the Collector instance. As a consequence, if the metrics service address contains an invalid port or uses environment variable expansion for the port, an error will be returned. A workaround for the environment variable case is to set `enableMetrics` to `false` and manually create the previously mentioned objects with the correct port if you need them.

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md).

In addition to the [core responsibilities](https://github.com/open-telemetry/community/blob/main/community-membership.md) the operator project requires approvers and maintainers to be responsible for releasing the project. See [RELEASE.md](./RELEASE.md) for more information and release schedule.

### Maintainers

- [Benedikt Bongartz](https://github.com/frzifus), Red Hat
- [Jacob Aronoff](https://github.com/jaronoff97), Omlet
- [Mikołaj Świątek](https://github.com/swiatekm), Elastic
- [Pavol Loffay](https://github.com/pavolloffay), Red Hat

For more information about the maintainer role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#maintainer).

### Approvers

- [Antoine Toulme](https://github.com/atoulme), Splunk
- [Israel Blancas](https://github.com/iblancasa), Coralogix
- [Tyler Helmuth](https://github.com/TylerHelmuth), Honeycomb
- [Yuri Oliveira Sa](https://github.com/yuriolisa), OllyGarden

For more information about the approver role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#approver).

### Triagers

For more information about the triager role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#triager).

### Emeritus Maintainers

- [Alex Boten](https://github.com/codeboten)
- [Bogdan Drutu](https://github.com/BogdanDrutu)
- [Juraci Paixão Kröhling](https://github.com/jpkrohling)
- [Tigran Najaryan](https://github.com/tigrannajaryan)
- [Vineeth Pothulapati](https://github.com/VineethReddy02)

For more information about the emeritus role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#emeritus-maintainerapprovertriager).

### Emeritus Approvers

- [Anthony Mirabella](https://github.com/Aneurysm9)
- [Dmitrii Anoshin](https://github.com/dmitryax)
- [James Bebbington](https://github.com/james-bebbington)
- [Jay Camp](https://github.com/jrcamp)
- [Owais Lone](https://github.com/owais)
- [Pablo Baeyens](https://github.com/mx-psi)

For more information about the emeritus role, see the [community repository](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#emeritus-maintainerapprovertriager).

Thanks to all the people who already contributed!

[![Contributors][contributors-img]][contributors]

## License

[Apache 2.0 License](./LICENSE).

[github-workflow]: https://github.com/open-telemetry/opentelemetry-operator/actions
[github-workflow-img]: https://github.com/open-telemetry/opentelemetry-operator/workflows/Continuous%20Integration/badge.svg
[goreport-img]: https://goreportcard.com/badge/github.com/open-telemetry/opentelemetry-operator
[goreport]: https://goreportcard.com/report/github.com/open-telemetry/opentelemetry-operator
[godoc-img]: https://godoc.org/github.com/open-telemetry/opentelemetry-operator?status.svg
[godoc]: https://godoc.org/github.com/open-telemetry/opentelemetry-operator/pkg/apis/opentelemetry/v1alpha1#OpenTelemetryCollector
[contributors]: https://github.com/open-telemetry/opentelemetry-operator/graphs/contributors
[contributors-img]: https://contributors-img.web.app/image?repo=open-telemetry/opentelemetry-operator
