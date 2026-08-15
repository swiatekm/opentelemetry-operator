// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"fmt"

	"github.com/buraksezer/consistent"
	"github.com/go-logr/logr"

	"github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/internal/target"
)

// strategyBuilder constructs a Strategy from the allocation strategy configuration. A builder reads only
// the configuration relevant to its strategy and constructs any strategies its strategy depends on
// (e.g. a fallback strategy), which are then injected into the strategy's constructor.
type strategyBuilder func(config StrategyConfig) (Strategy, error)

// strategyBuilders returns the registry of strategy builders keyed by strategy name. It is a function
// rather than a package-level variable so builders can call buildStrategy for the strategies they depend
// on without creating an initialization cycle.
func strategyBuilders() map[string]strategyBuilder {
	return map[string]strategyBuilder{
		leastWeightedStrategyName:     func(StrategyConfig) (Strategy, error) { return newleastWeightedStrategy(), nil },
		consistentHashingStrategyName: func(StrategyConfig) (Strategy, error) { return newConsistentHashingStrategy(), nil },
		perNodeStrategyName:           buildPerNodeStrategy,
	}
}

// buildStrategy constructs the named strategy, resolving and injecting any strategies it depends on.
func buildStrategy(name string, config StrategyConfig) (Strategy, error) {
	build, ok := strategyBuilders()[name]
	if !ok {
		return nil, fmt.Errorf("unregistered strategy: %s", name)
	}
	return build(config)
}

// buildFallbackStrategy constructs the named strategy for use as a fallback. Fallback strategies are
// built without any strategy configuration of their own, so a fallback strategy can never have a
// fallback itself. This keeps fallback chains bounded to a single level.
func buildFallbackStrategy(name string) (Strategy, error) {
	return buildStrategy(name, StrategyConfig{})
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
	// The fallback strategy is built with default options and can't have a fallback of its own.
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
	for s := range strategyBuilders() {
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
