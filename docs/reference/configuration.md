# Manager Configuration Reference

The OpenTelemetry Operator manager is configured via CLI flags, environment variables, or a YAML configuration file passed via `--config-file`.

## Configuration Precedence
When the same option is defined in multiple places, they are applied in this order of precedence (highest to lowest):
1. CLI flags
2. Environment variables
3. Configuration file

---

## Configuration Options

### General

#### `watch-namespace`
- **CLI Flag:** `--watch-namespace`
- **Env Variable:** `WATCH_NAMESPACE`
- **Type:** `string` (default: `""`)
- Comma-separated list of namespaces the operator should watch. An empty string watches all namespaces.

#### `enable-leader-election`
- **CLI Flag:** `--enable-leader-election`
- **Env Variable:** `ENABLE_LEADER_ELECTION`
- **Type:** `bool` (default: `false`)
- Enable leader election for the controller manager to ensure only one active manager replica runs at a time.

#### `enable-webhooks`
- **CLI Flag:** `--enable-webhooks`
- **Env Variable:** `ENABLE_WEBHOOKS`
- **Type:** `bool` (default: `true`)
- Enable admission webhooks used by the controllers.

#### `webhook-port`
- **CLI Flag:** `--webhook-port`
- **Env Variable:** `WEBHOOK_PORT`
- **Type:** `int` (default: `9443`)
- The port the webhook endpoint binds to.

#### `feature-gates`
- **CLI Flag:** `--feature-gates`
- **Env Variable:** `FEATURE_GATES`
- **Type:** `string` (default: `""`)
- Comma-separated list of feature gates to enable or disable (e.g., `gate1,-gate2`).

#### `ignore-missing-collector-crds`
- **CLI Flag:** `--ignore-missing-collector-crds`
- **Env Variable:** `IGNORE_MISSING_COLLECTOR_CRDS`
- **Type:** `bool` (default: `false`)
- Ignore the presence or absence of OpenTelemetryCollector CRDs in the cluster.

#### `create-rbac-permissions`
- **CLI Flag:** `--create-rbac-permissions`
- **Type:** `bool` (default: `false`)
- Automatically create RBAC permissions needed by the processors. (Deprecated)

#### `enable-instrumentation-crds`
- **Config Key:** `enable-instrumentation-crds`
- **Type:** `bool` (default: `true`)
- Enable looking up and validating Instrumentation CRDs in the cluster. (Only configurable via configuration file)

---

### Metrics

#### `metrics-addr`
- **CLI Flag:** `--metrics-addr`
- **Env Variable:** `METRICS_ADDR`
- **Type:** `string` (default: `":8443"`)
- The address the metrics endpoint binds to.

#### `metrics-secure`
- **CLI Flag:** `--metrics-secure`
- **Env Variable:** `METRICS_SECURE`
- **Type:** `bool` (default: `true`)
- Serve metrics securely via HTTPS with authentication/authorization.

#### `metrics-tls-cert-file`
- **CLI Flag:** `--metrics-tls-cert-file`
- **Env Variable:** `METRICS_TLS_CERT_FILE`
- **Type:** `string` (default: `""`)
- Path to the TLS certificate file for the metrics server.

#### `metrics-tls-key-file`
- **CLI Flag:** `--metrics-tls-key-file`
- **Env Variable:** `METRICS_TLS_KEY_FILE`
- **Type:** `string` (default: `""`)
- Path to the TLS private key file for the metrics server.

#### `enable-cr-metrics`
- **CLI Flag:** `--enable-cr-metrics`
- **Env Variable:** `ENABLE_CR_METRICS`
- **Type:** `bool` (default: `false`)
- Expose custom resource metrics.

#### `create-service-monitor-operator-metrics`
- **CLI Flag:** `--create-sm-operator-metrics`
- **Env Variable:** `CREATE_SM_OPERATOR_METRICS`
- **Type:** `bool` (default: `false`)
- Create a Prometheus Operator `ServiceMonitor` for the operator metrics.

---

### Health & Profiling

#### `health-probe-addr`
- **CLI Flag:** `--health-probe-addr`
- **Env Variable:** `HEALTH_PROBE_ADDR`
- **Type:** `string` (default: `":8081"`)
- The address the health probe endpoint binds to.

#### `pprof-addr`
- **CLI Flag:** `--pprof-addr`
- **Env Variable:** `PPROF_ADDR`
- **Type:** `string` (default: `""`)
- The address to expose the pprof profiling server. Empty disables pprof.

---

### TLS

#### `tls.useclusterprofile`
- **CLI Flag:** `--tls-cluster-profile`
- **Env Variable:** `TLS_CLUSTER_PROFILE`
- **Type:** `bool` (default: `false`)
- Retrieve the TLS profile from the cluster (OpenShift only).

#### `tls.configureoperands`
- **CLI Flag:** `--tls-configure-operands`
- **Env Variable:** `TLS_CONFIGURE_OPERANDS`
- **Type:** `bool` (default: `false`)
- Configures TLS in operands created by the operator.

#### `tls.minversion`
- **CLI Flag:** `--tls-min-version`
- **Env Variable:** `TLS_MIN_VERSION`
- **Type:** `string` (default: `"VersionTLS12"`)
- Minimum TLS version supported.

#### `tls.ciphersuites`
- **CLI Flag:** `--tls-cipher-suites`
- **Env Variable:** `TLS_CIPHER_SUITES`
- **Type:** `string slice` (default: `nil`)
- Allowed cipher suites. Comma-separated on CLI/Env, array in configuration file.

---

### Logging (Zap)

#### `zap.message-key`
- **CLI Flag:** `--zap-message-key`
- **Env Variable:** `ZAP_MESSAGE_KEY`
- **Type:** `string` (default: `"message"`)
- Message key for the Zap log encoder.

#### `zap.level-key`
- **CLI Flag:** `--zap-level-key`
- **Env Variable:** `ZAP_LEVEL_KEY`
- **Type:** `string` (default: `"level"`)
- Level key for the Zap log encoder.

#### `zap.time-key`
- **CLI Flag:** `--zap-time-key`
- **Env Variable:** `ZAP_TIME_KEY`
- **Type:** `string` (default: `"timestamp"`)
- Time key for the Zap log encoder.

#### `zap.level-format`
- **CLI Flag:** `--zap-level-format`
- **Env Variable:** `ZAP_LEVEL_FORMAT`
- **Type:** `string` (default: `"uppercase"`)
- Level format for the Zap log encoder.

---

### Filters

#### `labels-filter`
- **CLI Flag:** `--labels-filter`
- **Env Variable:** `LABELS_FILTER`
- **Type:** `string array` (default: `[]`)
- Labels to filter away from propagating onto deployed resources.

#### `annotations-filter`
- **CLI Flag:** `--annotations-filter`
- **Env Variable:** `ANNOTATIONS_FILTER`
- **Type:** `string array` (default: `["kubectl.kubernetes.io/last-applied-configuration"]`)
- Annotations to filter away from propagating onto deployed resources.

---

### Component Images

#### `collector-image`
- **CLI Flag:** `--collector-image`
- **Env Variable:** `RELATED_IMAGE_COLLECTOR`
- **Type:** `string` (default: dynamic)
- Default container image for the OpenTelemetry Collector.

#### `targetallocator-image`
- **CLI Flag:** `--target-allocator-image`
- **Env Variable:** `RELATED_IMAGE_TARGET_ALLOCATOR`
- **Type:** `string` (default: dynamic)
- Default container image for the OpenTelemetry Target Allocator.

#### `operatoropampbridge-image`
- **CLI Flag:** `--operator-opamp-bridge-image`
- **Env Variable:** `RELATED_IMAGE_OPERATOR_OPAMP_BRIDGE`
- **Type:** `string` (default: dynamic)
- Default container image for the OpAMP Bridge.

#### `collector-configmap-entry`
- **Config Key:** `collector-configmap-entry`
- **Type:** `string` (default: `"collector.yaml"`)
- Configuration file name for the collector. (Only configurable via configuration file)

#### `target-allocator-configmap-entry`
- **Config Key:** `target-allocator-configmap-entry`
- **Type:** `string` (default: `"targetallocator.yaml"`)
- Configuration file name for the Target Allocator. (Only configurable via configuration file)

#### `operator-op-amp-bridge-configmap-entry`
- **Config Key:** `operator-op-amp-bridge-configmap-entry`
- **Type:** `string` (default: `"remoteconfiguration.yaml"`)
- Configuration file name for the OpAMP Bridge. (Only configurable via configuration file)

---

### Auto-Instrumentation Images

#### `auto-instrumentation-java-image`
- **CLI Flag:** `--auto-instrumentation-java-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_JAVA`
- **Type:** `string` (default: dynamic)
- Container image for Java auto-instrumentation.

#### `auto-instrumentation-node-js-image`
- **CLI Flag:** `--auto-instrumentation-nodejs-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_NODEJS`
- **Type:** `string` (default: dynamic)
- Container image for NodeJS auto-instrumentation.

#### `auto-instrumentation-python-image`
- **CLI Flag:** `--auto-instrumentation-python-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_PYTHON`
- **Type:** `string` (default: dynamic)
- Container image for Python auto-instrumentation.

#### `auto-instrumentation-dot-net-image`
- **CLI Flag:** `--auto-instrumentation-dotnet-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_DOTNET`
- **Type:** `string` (default: dynamic)
- Container image for DotNet auto-instrumentation.

#### `auto-instrumentation-go-image`
- **CLI Flag:** `--auto-instrumentation-go-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_GO`
- **Type:** `string` (default: dynamic)
- Container image for Go auto-instrumentation.

#### `auto-instrumentation-apache-httpd-image`
- **CLI Flag:** `--auto-instrumentation-apache-httpd-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_APACHE_HTTPD`
- **Type:** `string` (default: dynamic)
- Container image for Apache HTTPD auto-instrumentation.

#### `auto-instrumentation-nginx-image`
- **CLI Flag:** `--auto-instrumentation-nginx-image`
- **Env Variable:** `RELATED_IMAGE_AUTO_INSTRUMENTATION_NGINX`
- **Type:** `string` (default: dynamic)
- Container image for Nginx auto-instrumentation.

---

### Auto-Instrumentation Feature Flags

#### `enable-multi-instrumentation`
- **CLI Flag:** `--enable-multi-instrumentation`
- **Env Variable:** `ENABLE_MULTI_INSTRUMENTATION`
- **Type:** `bool` (default: `true`)
- Controls whether the operator supports multi-instrumentation.

#### `enable-java-auto-instrumentation`
- **CLI Flag:** `--enable-java-instrumentation`
- **Env Variable:** `ENABLE_JAVA_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `true`)
- Enable Java auto-instrumentation.

#### `enable-node-js-auto-instrumentation`
- **CLI Flag:** `--enable-nodejs-instrumentation`
- **Env Variable:** `ENABLE_NODEJS_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `true`)
- Enable NodeJS auto-instrumentation.

#### `enable-python-auto-instrumentation`
- **CLI Flag:** `--enable-python-instrumentation`
- **Env Variable:** `ENABLE_PYTHON_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `true`)
- Enable Python auto-instrumentation.

#### `enable-dot-net-auto-instrumentation`
- **CLI Flag:** `--enable-dotnet-instrumentation`
- **Env Variable:** `ENABLE_DOTNET_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `true`)
- Enable DotNet auto-instrumentation.

#### `enable-apache-httpd-instrumentation`
- **CLI Flag:** `--enable-apache-httpd-instrumentation`
- **Env Variable:** `ENABLE_APACHE_HTTPD_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `true`)
- Enable Apache HTTPD auto-instrumentation.

#### `enable-go-auto-instrumentation`
- **CLI Flag:** `--enable-go-instrumentation`
- **Env Variable:** `ENABLE_GO_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `false`)
- Enable Go auto-instrumentation.

#### `enable-nginx-auto-instrumentation`
- **CLI Flag:** `--enable-nginx-instrumentation`
- **Env Variable:** `ENABLE_NGINX_AUTO_INSTRUMENTATION`
- **Type:** `bool` (default: `false`)
- Enable Nginx auto-instrumentation.

---

### OpenShift & FIPS

#### `openshift-create-dashboard`
- **CLI Flag:** `--openshift-create-dashboard`
- **Env Variable:** `OPENSHIFT_CREATE_DASHBOARD`
- **Type:** `bool` (default: `false`)
- Create OpenShift dashboard for monitoring collector instances.

#### `fips-disabled-components`
- **CLI Flag:** `--fips-disabled-components`
- **Env Variable:** `FIPS_DISABLED_COMPONENTS`
- **Type:** `string` (default: `"uppercase"`)
- Collector components to disable when running on a FIPS platform.

---

## Configuration File Structure

Structured YAML files use nested blocks rather than flat dotted keys for the `tls`, `zap`, and `instrumentations` properties:

```yaml
metrics-addr: :8443
metrics-secure: true
enable-leader-election: true

tls:
  minversion: VersionTLS12
  ciphersuites:
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384

zap:
  message-key: message
  time-key: timestamp
  level-key: level
  level-format: uppercase

instrumentations:
  spec:
    resource:
      resourceAttributes:
        service.namespace: "default"
```
