// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"fmt"
	"strings"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash/v2"

	"github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/internal/target"
)

const consistentHashingStrategyName = "consistent-hashing"

// labelValueSeparator separates label values in the hash key so that different label sets which would
// otherwise concatenate to the same string (e.g. ["ab", "c"] vs ["a", "bc"]) produce distinct keys.
const labelValueSeparator = 0xff

type hasher struct{}

func (hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

var _ Strategy = &consistentHashingStrategy{}

type consistentHashingStrategy struct {
	config           consistent.Config
	consistentHasher *consistent.Consistent
	// hashLabels are the target label names whose values are used to place a target on the hash ring.
	// When empty, the target's URL is used instead.
	hashLabels []string
}

func newConsistentHashingStrategy(hashLabels []string) Strategy {
	config := consistent.Config{
		PartitionCount:    1061,
		ReplicationFactor: 5,
		Load:              1.1,
		Hasher:            hasher{},
	}
	consistentHasher := consistent.New(nil, config)
	chStrategy := &consistentHashingStrategy{
		consistentHasher: consistentHasher,
		config:           config,
		hashLabels:       hashLabels,
	}
	return chStrategy
}

func (*consistentHashingStrategy) GetName() string {
	return consistentHashingStrategyName
}

func (s *consistentHashingStrategy) GetCollectorForTarget(collectors map[string]*Collector, item *target.Item) (*Collector, error) {
	member := s.consistentHasher.LocateKey(s.hashKey(item))
	collectorName := member.String()
	collector, ok := collectors[collectorName]
	if !ok {
		return nil, fmt.Errorf("unknown collector %s", collectorName)
	}
	return collector, nil
}

// hashKey returns the key used to place the target on the hash ring. By default the target's URL is used,
// which keeps a target on the same collector regardless of label changes. When hashLabels is configured,
// the values of those labels are used instead, so targets sharing those label values are assigned to the
// same collector.
func (s *consistentHashingStrategy) hashKey(item *target.Item) []byte {
	if len(s.hashLabels) == 0 {
		return []byte(item.TargetURL)
	}
	var sb strings.Builder
	for _, name := range s.hashLabels {
		sb.WriteString(item.Labels.Get(name))
		sb.WriteByte(labelValueSeparator)
	}
	return []byte(sb.String())
}

func (s *consistentHashingStrategy) SetCollectors(collectors map[string]*Collector) {
	// we simply recreate the hasher with the new member set
	// this isn't any more expensive than doing a diff and then applying the change
	var members []consistent.Member

	if len(collectors) > 0 {
		members = make([]consistent.Member, 0, len(collectors))
		for _, collector := range collectors {
			members = append(members, collector)
		}
	}

	s.consistentHasher = consistent.New(members, s.config)
}
