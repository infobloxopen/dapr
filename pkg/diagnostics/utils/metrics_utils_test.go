// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

func TestWithTags(t *testing.T) {
	t.Run("one tag", func(t *testing.T) {
		appKey := tag.MustNewKey("app_id")
		mutators := WithTags(appKey, "test")
		assert.Equal(t, 1, len(mutators))
	})

	t.Run("two tags", func(t *testing.T) {
		appKey := tag.MustNewKey("app_id")
		operationKey := tag.MustNewKey("operation")
		mutators := WithTags(appKey, "test", operationKey, "op")
		assert.Equal(t, 2, len(mutators))
	})

	t.Run("three tags", func(t *testing.T) {
		appKey := tag.MustNewKey("app_id")
		operationKey := tag.MustNewKey("operation")
		methodKey := tag.MustNewKey("method")
		mutators := WithTags(appKey, "test", operationKey, "op", methodKey, "method")
		assert.Equal(t, 3, len(mutators))
	})

	t.Run("two tags with wrong value type", func(t *testing.T) {
		appKey := tag.MustNewKey("app_id")
		operationKey := tag.MustNewKey("operation")
		mutators := WithTags(appKey, "test", operationKey, 1)
		assert.Equal(t, 1, len(mutators))
	})

	t.Run("skip empty value key", func(t *testing.T) {
		appKey := tag.MustNewKey("app_id")
		operationKey := tag.MustNewKey("operation")
		methodKey := tag.MustNewKey("method")
		mutators := WithTags(appKey, "", operationKey, "op", methodKey, "method")
		assert.Equal(t, 2, len(mutators))
	})
}

func TestNewMeasureView(t *testing.T) {
	t.Run("creates view with distribution aggregation", func(t *testing.T) {
		m := stats.Int64("test/measure", "test measure", stats.UnitDimensionless)
		keys := []tag.Key{tag.MustNewKey("test_key")}
		agg := view.Distribution(0, 100, 200)

		v := NewMeasureView(m, keys, agg)
		require.NotNil(t, v)
		assert.Equal(t, "test/measure", v.Name)
		assert.Equal(t, "test measure", v.Description)
		assert.Equal(t, m, v.Measure)
		assert.Equal(t, keys, v.TagKeys)
	})

	t.Run("creates view with count aggregation", func(t *testing.T) {
		m := stats.Int64("test/count", "test count", stats.UnitDimensionless)
		v := NewMeasureView(m, nil, view.Count())
		require.NotNil(t, v)
		assert.Equal(t, "test/count", v.Name)
	})

	t.Run("creates view with last value aggregation", func(t *testing.T) {
		m := stats.Float64("test/lastvalue", "test last value", stats.UnitDimensionless)
		v := NewMeasureView(m, nil, view.LastValue())
		require.NotNil(t, v)
		assert.Equal(t, "test/lastvalue", v.Name)
	})
}

func TestAddTagKeyToCtx(t *testing.T) {
	t.Run("adds tag key to context", func(t *testing.T) {
		ctx := context.Background()
		key := tag.MustNewKey("test_key")
		newCtx := AddTagKeyToCtx(ctx, key, "test_value")
		assert.NotNil(t, newCtx)
	})

	t.Run("empty value returns original context", func(t *testing.T) {
		ctx := context.Background()
		key := tag.MustNewKey("test_key_empty")
		newCtx := AddTagKeyToCtx(ctx, key, "")
		assert.Equal(t, ctx, newCtx)
	})
}

func TestAddNewTagKey(t *testing.T) {
	t.Run("adds tag key to views", func(t *testing.T) {
		m := stats.Int64("test/addkey", "test", stats.UnitDimensionless)
		v1 := &view.View{
			Name:        m.Name(),
			Measure:     m,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{},
		}
		v2 := &view.View{
			Name:        "test/addkey2",
			Measure:     m,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{},
		}

		newKey := tag.MustNewKey("new_key")
		views := AddNewTagKey([]*view.View{v1, v2}, &newKey)

		assert.Len(t, views, 2)
		assert.Contains(t, views[0].TagKeys, newKey)
		assert.Contains(t, views[1].TagKeys, newKey)
	})
}
