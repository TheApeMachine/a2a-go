package stores

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInMemorySessionStore(t *testing.T) {
	store := NewInMemorySessionStore()
	assert.NotNil(t, store)
	assert.NotNil(t, store.data)
	assert.Empty(t, store.data)
}

func TestSessionStore_Get(t *testing.T) {
	store := NewInMemorySessionStore()

	// Test getting non-existent session
	data, exists := store.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, data)

	// Set some data
	testData := map[string]any{
		"key1": "value1",
		"key2": 123,
	}
	store.Set("session1", testData)

	// Test getting existing session
	data, exists = store.Get("session1")
	assert.True(t, exists)
	assert.Equal(t, testData, data)
}

func TestSessionStore_Set(t *testing.T) {
	store := NewInMemorySessionStore()

	// Test setting new session
	testData := map[string]any{
		"key1": "value1",
		"key2": 123,
	}
	store.Set("session1", testData)

	// Verify data was stored
	data, exists := store.Get("session1")
	assert.True(t, exists)
	assert.Equal(t, testData, data)

	// Test overwriting existing session
	newData := map[string]any{
		"key3": "value3",
	}
	store.Set("session1", newData)

	// Verify data was updated
	data, exists = store.Get("session1")
	assert.True(t, exists)
	assert.Equal(t, newData, data)
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewInMemorySessionStore()

	// Set some data
	testData := map[string]any{
		"key1": "value1",
	}
	store.Set("session1", testData)

	// Verify data exists
	_, exists := store.Get("session1")
	assert.True(t, exists)

	// Delete the session
	store.Delete("session1")

	// Verify data was deleted
	_, exists = store.Get("session1")
	assert.False(t, exists)

	// Test deleting non-existent session (should not panic)
	store.Delete("nonexistent")
}
