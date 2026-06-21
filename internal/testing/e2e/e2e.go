// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/prometheus/common/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	k8sobj "sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const fieldManager = "opentelemetry-operator-e2e"

// crClient builds a controller-runtime client (typed + unstructured, with a dynamic
// RESTMapper for CRDs) from the test's REST config.
func crClient(t *testing.T, cfg *envconf.Config) crclient.Client {
	t.Helper()
	c, err := crclient.New(cfg.Client().RESTConfig(), crclient.Options{Scheme: clientgoscheme.Scheme})
	if err != nil {
		t.Fatalf("controller-runtime client: %v", err)
	}
	return c
}

// clientSet builds a client-go clientset, used for the API server service proxy.
func clientSet(t *testing.T, cfg *envconf.Config) *kubernetes.Clientset {
	t.Helper()
	cs, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatalf("clientset: %v", err)
	}
	return cs
}

// Apply server-side-applies multi-document YAML into ns (every object is namespaced
// into ns). Objects are decoded as unstructured, so no scheme registration is needed
// for CRDs like OpenTelemetryCollector, Prometheus or ServiceMonitor.
func Apply(ctx context.Context, t *testing.T, cfg *envconf.Config, ns, manifests string) {
	t.Helper()
	applyManifests(ctx, t, crClient(t, cfg), strings.NewReader(manifests), ns)
}

// applyManifests SSA-applies each document from r. When forceNS is non-empty it is set
// as the namespace on every object (callers pass it only for namespaced manifests);
// when empty, each object's own namespace (if any) is respected.
func applyManifests(ctx context.Context, t *testing.T, c crclient.Client, r io.Reader, forceNS string) {
	t.Helper()
	dec := utilyaml.NewYAMLOrJSONDecoder(r, 4096)
	for {
		raw := map[string]any{}
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("decode manifest: %v", err)
		}
		if len(raw) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: raw}
		if forceNS != "" {
			u.SetNamespace(forceNS)
		}
		if err := c.Patch(ctx, u, crclient.Apply, crclient.FieldOwner(fieldManager), crclient.ForceOwnership); err != nil {
			t.Fatalf("apply %s %q: %v", u.GetKind(), u.GetName(), err)
		}
	}
}

// CreateNamespace creates ns.
func CreateNamespace(ctx context.Context, t *testing.T, cfg *envconf.Config, ns string) {
	t.Helper()
	obj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	if err := crClient(t, cfg).Create(ctx, obj); err != nil {
		t.Fatalf("create namespace %s: %v", ns, err)
	}
}

// DeleteNamespace deletes ns (ignoring not-found), used for test cleanup.
func DeleteNamespace(ctx context.Context, t *testing.T, cfg *envconf.Config, ns string) {
	t.Helper()
	obj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	if err := crClient(t, cfg).Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("delete namespace %s: %v", ns, err)
	}
}

// WaitForStatefulSet blocks until the named StatefulSet reports >= replicas ready.
func WaitForStatefulSet(ctx context.Context, t *testing.T, cfg *envconf.Config, ns, name string, replicas int32, timeout time.Duration) {
	t.Helper()
	ss := &appsv1.StatefulSet{}
	ss.SetName(name)
	ss.SetNamespace(ns)
	if err := wait.For(
		conditions.New(cfg.Client().Resources()).ResourceMatch(ss, func(obj k8sobj.Object) bool {
			return obj.(*appsv1.StatefulSet).Status.ReadyReplicas >= replicas
		}),
		wait.WithContext(ctx),
		wait.WithTimeout(timeout),
		wait.WithInterval(2*time.Second),
	); err != nil {
		t.Fatalf("statefulset %s/%s not ready: %v", ns, name, err)
	}
}

// WaitForDeployment blocks until the named Deployment reports Available.
func WaitForDeployment(ctx context.Context, t *testing.T, cfg *envconf.Config, ns, name string, timeout time.Duration) {
	t.Helper()
	dep := &appsv1.Deployment{}
	dep.SetName(name)
	dep.SetNamespace(ns)
	if err := wait.For(
		conditions.New(cfg.Client().Resources()).DeploymentConditionMatch(dep, appsv1.DeploymentAvailable, corev1.ConditionTrue),
		wait.WithContext(ctx),
		wait.WithTimeout(timeout),
		wait.WithInterval(2*time.Second),
	); err != nil {
		t.Fatalf("deployment %s/%s not available: %v", ns, name, err)
	}
}

// PromTarget identifies a Prometheus backend by its in-namespace Service and port.
type PromTarget struct {
	Service string
	Port    int
}

// queryProm runs an instant PromQL query against a Prometheus Service via the API
// server's service proxy (works for headless Services, so no port-forward is needed).
func queryProm(ctx context.Context, cs *kubernetes.Clientset, ns string, target PromTarget, query string) (model.Vector, error) {
	raw, err := cs.CoreV1().Services(ns).
		ProxyGet("http", target.Service, strconv.Itoa(target.Port), "/api/v1/query", map[string]string{"query": query}).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	return parsePromVector(raw)
}

// parsePromVector decodes a Prometheus instant-query JSON response into a model.Vector.
func parsePromVector(raw []byte) (model.Vector, error) {
	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", resp.Error)
	}
	if resp.Data.ResultType != "vector" {
		return nil, fmt.Errorf("query returned %q, want a vector", resp.Data.ResultType)
	}
	var vec model.Vector
	for _, r := range resp.Data.Result {
		metric := model.Metric{}
		for k, v := range r.Metric {
			metric[model.LabelName(k)] = model.LabelValue(v)
		}
		var val model.SampleValue
		if len(r.Value) == 2 {
			s, _ := r.Value[1].(string)
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("parse sample value %q: %w", s, err)
			}
			val = model.SampleValue(f)
		}
		vec = append(vec, &model.Sample{Metric: metric, Value: val})
	}
	return vec, nil
}

// eventually retries fn until it returns nil, ctx is done, or attempts are exhausted.
func eventually(ctx context.Context, t *testing.T, desc string, attempts int, interval time.Duration, fn func() error) {
	t.Helper()
	var err error
	for range attempts {
		if err = fn(); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s: %v (last error: %v)", desc, ctx.Err(), err)
		case <-time.After(interval):
		}
	}
	t.Fatalf("%s: gave up after %d attempts: %v", desc, attempts, err)
}

// EventuallyPromQL runs an instant query against a Prometheus Service and retries until
// check passes. The check is plain Go over a model.Vector — assert on values and labels
// of the queried series.
func EventuallyPromQL(ctx context.Context, t *testing.T, cfg *envconf.Config, ns string, target PromTarget, query string, check func(model.Vector) error) {
	t.Helper()
	cs := clientSet(t, cfg)
	eventually(ctx, t, "promql: "+query, 60, 2*time.Second, func() error {
		vec, err := queryProm(ctx, cs, ns, target, query)
		if err != nil {
			return err
		}
		return check(vec)
	})
}

// SameLabelsAcross is a differential within one Prometheus: it queries a single
// backend, partitions the result series by the value of partitionLabel, and asserts
// every partition exposes the same set of label-sets (after dropping partitionLabel
// and any ignore labels). Use it to compare two pipelines that write to one
// Prometheus distinguished by a label — e.g. the target allocator pipeline
// (pipeline=ta) versus prometheus-operator scraping the same target natively
// (pipeline=oracle). Identical sets mean the allocator labeled the target exactly as
// prometheus-operator does. It retries until at least wantPartitions distinct
// partitions are present and they agree.
//
// Query `up` to compare pure target identity. Pass ignore for labels that
// legitimately differ between pipelines.
func SameLabelsAcross(ctx context.Context, t *testing.T, cfg *envconf.Config, ns string, target PromTarget, query, partitionLabel string, wantPartitions int, ignore ...string) {
	t.Helper()
	cs := clientSet(t, cfg)
	drop := map[string]bool{"__name__": true, partitionLabel: true}
	for _, l := range ignore {
		drop[l] = true
	}

	desc := fmt.Sprintf("differential %q across %q", query, partitionLabel)
	eventually(ctx, t, desc, 60, 2*time.Second, func() error {
		vec, err := queryProm(ctx, cs, ns, target, query)
		if err != nil {
			return err
		}
		parts := map[string]map[string]bool{} // partition value -> set of canonical label-sets
		for _, s := range vec {
			pv := string(s.Metric[model.LabelName(partitionLabel)])
			labels := map[string]string{}
			for k, v := range s.Metric {
				if drop[string(k)] {
					continue
				}
				labels[string(k)] = string(v)
			}
			if parts[pv] == nil {
				parts[pv] = map[string]bool{}
			}
			parts[pv][canonicalLabels(labels)] = true
		}
		if len(parts) < wantPartitions {
			return fmt.Errorf("waiting for %d partitions of %q, have %d: %v", wantPartitions, partitionLabel, len(parts), slices.Sorted(maps.Keys(parts)))
		}
		var refName string
		var ref map[string]bool
		for name, set := range parts {
			if ref == nil {
				refName, ref = name, set
				continue
			}
			if diff := diffSets(ref, set); diff != "" {
				return fmt.Errorf("label sets differ between %s=%q (A) and %s=%q (B):\n%s", partitionLabel, refName, partitionLabel, name, diff)
			}
		}
		return nil
	})
}

// canonicalLabels renders a label set as a single deterministic string (labels sorted
// by name, each rendered as name="value") so that label sets can be used as map keys
// and compared for set membership.
func canonicalLabels(m map[string]string) string {
	var b strings.Builder
	for i, k := range slices.Sorted(maps.Keys(m)) {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s=%q", k, m[k])
	}
	return b.String()
}

// diffSets returns a human-readable description of the symmetric difference between
// two sets of canonical label-sets — the members only in a ("only in A") and only in
// b ("only in B"). It returns the empty string when the sets are equal.
func diffSets(a, b map[string]bool) string {
	var onlyA, onlyB []string
	for k := range a {
		if !b[k] {
			onlyA = append(onlyA, k)
		}
	}
	for k := range b {
		if !a[k] {
			onlyB = append(onlyB, k)
		}
	}
	if len(onlyA) == 0 && len(onlyB) == 0 {
		return ""
	}
	slices.Sort(onlyA)
	slices.Sort(onlyB)
	var sb strings.Builder
	for _, s := range onlyA {
		fmt.Fprintf(&sb, "  only in A: {%s}\n", s)
	}
	for _, s := range onlyB {
		fmt.Fprintf(&sb, "  only in B: {%s}\n", s)
	}
	return sb.String()
}

// Series is an expected Prometheus series for HasSeries. A sample matches when it
// carries every label in Labels (exact value), carries every label in Present (any
// value), carries no label in Absent, and — when Value is non-nil — its value
// satisfies it. When Exact is true the sample must carry EXACTLY the union of Labels
// and Present and nothing else, which pins a target's complete label set while still
// allowing non-deterministic values (e.g. a pod name) via Present.
//
// Absent and Exact are how a test proves target labeling: a static scrape config
// must carry no Kubernetes service-discovery labels, while a ServiceMonitor target
// must carry exactly the relabeled identity the allocator generated.
type Series struct {
	Labels  map[string]string
	Present []string
	Absent  []string
	Exact   bool
	Value   func(model.SampleValue) bool
}

// HasSeries returns a PromQL check (for EventuallyPromQL) that passes when some
// sample in the result vector matches want.
func HasSeries(want Series) func(model.Vector) error {
	return func(v model.Vector) error {
		for _, s := range v {
			if seriesMatches(want, s) {
				return nil
			}
		}
		return fmt.Errorf("no sample matched labels=%v absent=%v among %d samples: %v",
			want.Labels, want.Absent, len(v), v)
	}
}

func seriesMatches(want Series, s *model.Sample) bool {
	for k, val := range want.Labels {
		if string(s.Metric[model.LabelName(k)]) != val {
			return false
		}
	}
	for _, k := range want.Present {
		if _, ok := s.Metric[model.LabelName(k)]; !ok {
			return false
		}
	}
	for _, k := range want.Absent {
		if _, ok := s.Metric[model.LabelName(k)]; ok {
			return false
		}
	}
	if want.Exact {
		allowed := map[string]bool{"__name__": true}
		for k := range want.Labels {
			allowed[k] = true
		}
		for _, k := range want.Present {
			allowed[k] = true
		}
		for k := range s.Metric {
			if !allowed[string(k)] {
				return false
			}
		}
	}
	return want.Value == nil || want.Value(s.Value)
}

// Equals matches a sample value exactly.
func Equals(want model.SampleValue) func(model.SampleValue) bool {
	return func(got model.SampleValue) bool { return got == want }
}

// AtLeast matches a sample value >= want.
func AtLeast(want model.SampleValue) func(model.SampleValue) bool {
	return func(got model.SampleValue) bool { return got >= want }
}

// BindTargetAllocatorClusterRole applies the project's shipped ClusterRole
// (config/target-allocator/clusterrole.yaml — core target discovery plus the
// Prometheus CRDs) and binds it to the named ServiceAccount in ns. The ClusterRole is
// cluster-scoped and shared (server-side-applied); the per-test ClusterRoleBinding is
// removed on cleanup. It is reused for both the target allocator and an
// operator-managed oracle Prometheus, which needs the same core discovery access.
//
// The operator deliberately does not create the allocator's RBAC itself — the
// permissions a target allocator needs depend on what the user asks it to discover —
// so an e2e test that runs the allocator must supply them.
func BindTargetAllocatorClusterRole(ctx context.Context, t *testing.T, cfg *envconf.Config, ns, saName string) {
	t.Helper()
	c := crClient(t, cfg)

	clusterRole, err := os.Open(filepath.Join(RepoRoot(t), "config", "target-allocator", "clusterrole.yaml"))
	if err != nil {
		t.Fatalf("open clusterrole.yaml: %v", err)
	}
	defer clusterRole.Close()
	applyManifests(ctx, t, c, clusterRole, "")

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: ns + "-" + saName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "target-allocator",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      saName,
			Namespace: ns,
		}},
	}
	if err := c.Create(ctx, binding); err != nil {
		t.Fatalf("create clusterrolebinding %s: %v", binding.Name, err)
	}
	t.Cleanup(func() {
		if err := c.Delete(context.WithoutCancel(ctx), binding); err != nil && !apierrors.IsNotFound(err) {
			t.Errorf("delete clusterrolebinding %s: %v", binding.Name, err)
		}
	})
}

// EnsurePrometheusOperator installs prometheus-operator into the cluster if its
// controller is not already running, so tests can use a Prometheus CR as a live
// oracle. The version matches the prometheus-operator module this repo depends on
// (see prometheusOperatorVersion). Idempotent: a no-op once installed. The release
// bundle is fetched over the network and server-side-applied.
func EnsurePrometheusOperator(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	t.Helper()
	c := crClient(t, cfg)
	dep := &appsv1.Deployment{}
	err := c.Get(ctx, crclient.ObjectKey{Namespace: "default", Name: "prometheus-operator"}, dep)
	if err == nil {
		return
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get prometheus-operator deployment: %v", err)
	}

	url := "https://github.com/prometheus-operator/prometheus-operator/releases/download/" + prometheusOperatorVersion(t) + "/bundle.yaml"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("request bundle: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fetch bundle: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch bundle: status %d", resp.StatusCode)
	}
	// The bundle mixes cluster-scoped (CRDs, RBAC) and namespaced (operator in
	// default) objects, so its own namespaces are respected.
	applyManifests(ctx, t, c, resp.Body, "")
	WaitForDeployment(ctx, t, cfg, "default", "prometheus-operator", 3*time.Minute)
}

// prometheusOperatorVersion returns the prometheus-operator version this repo depends
// on, read from go.mod, so the installed operator matches the API types and CRDs the
// code is written against.
func prometheusOperatorVersion(t *testing.T) string {
	t.Helper()
	const module = "github.com/prometheus-operator/prometheus-operator"
	path := filepath.Join(RepoRoot(t), "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		t.Fatalf("parse go.mod: %v", err)
	}
	for _, r := range f.Require {
		// Match the module's own require, not its /pkg/... submodules.
		if r.Mod.Path == module {
			return r.Mod.Version
		}
	}
	t.Fatalf("module %s not found in go.mod", module)
	return ""
}

// RepoRoot walks up from the test's working directory to the repository root,
// identified by config/target-allocator/clusterrole.yaml. It lets framework helpers
// reference shipped manifests regardless of the calling test package's depth.
func RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "config", "target-allocator", "clusterrole.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repo root (config/target-allocator) from %s", dir)
		}
		dir = parent
	}
}
