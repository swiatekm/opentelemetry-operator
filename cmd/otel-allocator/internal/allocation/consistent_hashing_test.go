// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package allocation

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/internal/target"
)

// TestConsistentHashingByLabels builds the strategy through the factory (exercising the full
// config -> builder -> constructor path) and verifies that when hashing labels are configured, targets
// sharing those label values are assigned to the same collector regardless of their URL.
func TestConsistentHashingByLabels(t *testing.T) {
	allocator, err := New(consistentHashingStrategyName, logger, WithStrategyConfig(StrategyConfig{
		ConsistentHashing: ConsistentHashingStrategyConfig{Labels: []string{"shard"}},
	}))
	require.NoError(t, err)

	allocator.SetCollectors(MakeNCollectors(10, 0))

	// Two targets share the same "shard" value but differ in URL and in an unrelated label.
	labelsA := labels.New(labels.Label{Name: "shard", Value: "a"}, labels.Label{Name: "pod", Value: "1"})
	labelsB := labels.New(labels.Label{Name: "shard", Value: "a"}, labels.Label{Name: "pod", Value: "2"})
	targetA := target.NewItem("job", "10.0.0.1:8080", labelsA, "", target.HashLabels(labelsA, "job"))
	targetB := target.NewItem("job", "10.0.0.2:9090", labelsB, "", target.HashLabels(labelsB, "job"))

	allocator.SetTargets([]*target.Item{targetA, targetB})

	items := allocator.TargetItems()
	itemA, ok := items[targetA.Hash()]
	require.True(t, ok)
	itemB, ok := items[targetB.Hash()]
	require.True(t, ok)

	require.NotEmpty(t, itemA.CollectorName)
	assert.Equal(t, itemA.CollectorName, itemB.CollectorName,
		"targets sharing the configured hashing label should be assigned to the same collector regardless of URL")
}

func TestRelativelyEvenDistribution(t *testing.T) {
	numCols := 15
	numItems := 10000
	cols := MakeNCollectors(numCols, 0)
	expectedPerCollector := float64(numItems / numCols)
	expectedDelta := (expectedPerCollector * 1.5) - expectedPerCollector
	c, _ := New("consistent-hashing", logger)
	c.SetCollectors(cols)
	c.SetTargets(MakeNNewTargets(numItems, 0, 0))
	actualTargetItems := c.TargetItems()
	assert.Len(t, actualTargetItems, numItems)
	actualCollectors := c.Collectors()
	assert.Len(t, actualCollectors, numCols)
	for _, col := range actualCollectors {
		assert.InDelta(t, col.NumTargets, expectedPerCollector, expectedDelta)
	}
}

func TestFullReallocation(t *testing.T) {
	cols := MakeNCollectors(10, 0)
	c, _ := New("consistent-hashing", logger)
	c.SetCollectors(cols)
	c.SetTargets(MakeNNewTargets(10000, 10, 0))
	actualTargetItems := c.TargetItems()
	assert.Len(t, actualTargetItems, 10000)
	actualCollectors := c.Collectors()
	assert.Len(t, actualCollectors, 10)
	newCols := MakeNCollectors(10, 10)
	c.SetCollectors(newCols)
	updatedTargetItems := c.TargetItems()
	assert.Len(t, updatedTargetItems, 10000)
	updatedCollectors := c.Collectors()
	assert.Len(t, updatedCollectors, 10)
	for _, item := range updatedTargetItems {
		_, ok := updatedCollectors[item.CollectorName]
		assert.True(t, ok, "Some items weren't reallocated correctly")
	}
}

func TestNumRemapped(t *testing.T) {
	numItems := 10_000
	numInitialCols := 15
	numFinalCols := 16
	expectedDelta := float64((numFinalCols - numInitialCols) * (numItems / numFinalCols))
	cols := MakeNCollectors(numInitialCols, 0)
	c, _ := New("consistent-hashing", logger)
	c.SetCollectors(cols)
	c.SetTargets(MakeNNewTargets(numItems, numInitialCols, 0))
	actualTargetItems := c.TargetItems()
	assert.Len(t, actualTargetItems, numItems)
	actualCollectors := c.Collectors()
	assert.Len(t, actualCollectors, numInitialCols)
	newCols := MakeNCollectors(numFinalCols, 0)
	c.SetCollectors(newCols)
	updatedTargetItems := c.TargetItems()
	assert.Len(t, updatedTargetItems, numItems)
	updatedCollectors := c.Collectors()
	assert.Len(t, updatedCollectors, numFinalCols)
	countRemapped := 0
	countNotRemapped := 0
	for _, item := range updatedTargetItems {
		previousItem, ok := actualTargetItems[item.Hash()]
		assert.True(t, ok)
		if previousItem.CollectorName != item.CollectorName {
			countRemapped++
		} else {
			countNotRemapped++
		}
	}
	assert.InDelta(t, numItems/numFinalCols, countRemapped, expectedDelta)
}

func TestTargetsWithNoCollectorsConsistentHashing(t *testing.T) {
	c, _ := New("consistent-hashing", logger)

	// Adding 10 new targets
	numItems := 10
	c.SetTargets(MakeNNewTargetsWithEmptyCollectors(numItems, 0))
	actualTargetItems := c.TargetItems()
	assert.Len(t, actualTargetItems, numItems)

	// Adding 5 new targets, and removing the old 10 targets
	numItemsUpdate := 5
	c.SetTargets(MakeNNewTargetsWithEmptyCollectors(numItemsUpdate, 10))
	actualTargetItemsUpdated := c.TargetItems()
	assert.Len(t, actualTargetItemsUpdated, numItemsUpdate)

	// Adding 5 new targets, and one existing target
	numItemsUpdate = 6
	c.SetTargets(MakeNNewTargetsWithEmptyCollectors(numItemsUpdate, 14))
	actualTargetItemsUpdated = c.TargetItems()
	assert.Len(t, actualTargetItemsUpdated, numItemsUpdate)

	// Adding collectors to test allocation
	numCols := 2
	cols := MakeNCollectors(2, 0)
	c.SetCollectors(cols)
	expectedPerCollector := float64(numItemsUpdate / numCols)
	expectedDelta := (expectedPerCollector * 1.5) - expectedPerCollector
	// Checking to see that there is no change to number of targets
	actualTargetItems = c.TargetItems()
	assert.Len(t, actualTargetItems, numItemsUpdate)
	// Checking to see collectors are added correctly
	actualCollectors := c.Collectors()
	assert.Len(t, actualCollectors, numCols)
	for _, col := range actualCollectors {
		assert.InDelta(t, col.NumTargets, expectedPerCollector, expectedDelta)
	}
}
