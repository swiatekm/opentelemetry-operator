// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"fmt"

	"github.com/buraksezer/consistent"
	"github.com/go-logr/logr"

	"github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/internal/target"
)

// resolveStrategy builds a strategy by name. It is passed to strategy builders so they can construct the
// strategies they depend on without referring to package-level state, which would otherwise create an
// initialization cycle with strategyBuilders.
type resolveStrategy func(name string, config StrategyConfig) (Strategy, error)

// strategyBuilder constructs a Strategy from the allocation strategy configuration. A builder reads only
// the configuration relevant to its strategy and uses resolve to construct any strategies its strategy
// depends on (e.g. a fallback strategy), which are then injected into the strategy's constructor.
type strategyBuilder func(config StrategyConfig, resolve resolveStrategy) (Strategy, error)

var strategyBuilders = map[string]strategyBuilder{
	leastWeightedStrategyName:     func(StrategyConfig, resolveStrategy) (Strategy, error) { return newleastWeightedStrategy(), nil },
	consistentHashingStrategyName: func(StrategyConfig, resolveStrategy) (Strategy, error) { return newConsistentHashingStrategy(), nil },
	perNodeStrategyName:           buildPerNodeStrategy,
}

// buildStrategy constructs the named strategy, resolving and injecting any strategies it depends on.
func buildStrategy(name string, config StrategyConfig) (Strategy, error) {
	build, ok := strategyBuilders[name]
	if !ok {
		return nil, fmt.Errorf("unregistered strategy: %s", name)
	}
	return build(config, buildStrategy)
}

// Option configures the allocator constructed by New.
type Option func(*allocatorOptions)

type allocatorOptions struct {
	strategyConfig StrategyConfig
}

// StrategyConfig holds the configuration for the allocation strategies. Each strategy has its own
// section because strategies accept different configuration options.
type StrategyConfig struct {
	PerNode PerNodeStrategyConfig
}

// PerNodeStrategyConfig holds the configuration options for the per-node strategy.
type PerNodeStrategyConfig struct {
	// FallbackStrategy is the name of the strategy used for targets the per-node strategy can't assign on
	// its own, for example targets which don't have a node label. If empty, such targets are left unassigned.
	FallbackStrategy string
}

// WithStrategyConfig sets the configuration used to construct the allocator's strategy.
func WithStrategyConfig(config StrategyConfig) Option {
	return func(o *allocatorOptions) {
		o.strategyConfig = config
	}
}

func New(name string, log logr.Logger, opts ...Option) (Allocator, error) {
	var options allocatorOptions
	for _, opt := range opts {
		opt(&options)
	}
	strategy, err := buildStrategy(name, options.strategyConfig)
	if err != nil {
		return nil, err
	}
	return newAllocator(log.WithValues("allocator", name), strategy)
}

func GetRegisteredAllocatorNames() []string {
	var names []string
	for s := range strategyBuilders {
		names = append(names, s)
	}
	return names
}

type Allocator interface {
	SetCollectors(collectors map[string]*Collector)
	SetTargets(targets []*target.Item)
	TargetItems() map[target.ItemHash]*target.Item
	Collectors() map[string]*Collector
	GetTargetsForCollectorAndJob(collector, job string) []*target.Item
}

type Strategy interface {
	GetCollectorForTarget(map[string]*Collector, *target.Item) (*Collector, error)
	// SetCollectors exists for strategies where changing the collector set is potentially an expensive operation.
	// The caller must guarantee that the collectors map passed in GetCollectorForTarget is consistent with the latest
	// SetCollectors call. Strategies which don't need this information can just ignore it.
	SetCollectors(map[string]*Collector)
	GetName() string
}

var _ consistent.Member = Collector{}

// Collector Creates a struct that holds Collector information.
// This struct will be parsed into endpoint with Collector and jobs info.
// This struct can be extended with information like annotations and labels in the future.
type Collector struct {
	Name          string
	NodeName      string
	NumTargets    int
	TargetsPerJob map[string]int
}

func (c Collector) Hash() string {
	return c.Name
}

func (c Collector) String() string {
	return c.Name
}

func NewCollector(name, node string) *Collector {
	return &Collector{Name: name, NodeName: node, TargetsPerJob: make(map[string]int)}
}
