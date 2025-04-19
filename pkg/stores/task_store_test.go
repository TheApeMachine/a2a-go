package stores

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestNewInMemoryTaskStore(t *testing.T) {
	store := NewInMemoryTaskStore()
	assert.NotNil(t, store)
	assert.NotNil(t, store.tasks)
	assert.Empty(t, store.tasks)
}

func TestTaskStore_Create(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Test creating a new task
	task := store.Create("task1", "Test Task")
	assert.NotNil(t, task)
	assert.Equal(t, "task1", task.ID)
	assert.Equal(t, "Test Task", task.Description)
	assert.Equal(t, types.TaskStateSubmitted, task.State)
	assert.Nil(t, task.ParentID)
	assert.NotZero(t, task.CreatedAt)
	assert.NotZero(t, task.UpdatedAt)
	assert.Empty(t, task.History)
	assert.Nil(t, task.PushNotification)
}

func TestTaskStore_CreateChild(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create parent task
	parent := store.Create("parent1", "Parent Task")

	// Create child task
	child := store.CreateChild("child1", "Child Task", parent.ID)
	assert.NotNil(t, child)
	assert.Equal(t, "child1", child.ID)
	assert.Equal(t, "Child Task", child.Description)
	assert.Equal(t, parent.ID, *child.ParentID)
}

func TestTaskStore_Get(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	store.Create("task1", "Test Task")

	// Test getting existing task
	task, exists := store.Get("task1")
	assert.True(t, exists)
	assert.NotNil(t, task)
	assert.Equal(t, "task1", task.ID)

	// Test getting non-existent task
	task, exists = store.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, task)
}

func TestTaskStore_UpdateState(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	store.Create("task1", "Test Task")

	// Test updating state
	success := store.UpdateState("task1", types.TaskStateWorking)
	assert.True(t, success)

	task, _ := store.Get("task1")
	assert.Equal(t, types.TaskStateWorking, task.State)

	// Test updating non-existent task
	success = store.UpdateState("nonexistent", types.TaskStateWorking)
	assert.False(t, success)
}

func TestTaskStore_List(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create multiple tasks
	store.Create("task1", "Task 1")
	store.Create("task2", "Task 2")

	// Test listing tasks
	tasks := store.List()
	assert.Len(t, tasks, 2)

	// Verify task contents
	taskMap := make(map[string]bool)
	for _, task := range tasks {
		taskMap[task.ID] = true
	}
	assert.True(t, taskMap["task1"])
	assert.True(t, taskMap["task2"])
}

func TestTaskStore_AddMessageToHistory(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	store.Create("task1", "Test Task")

	// Test adding message
	msg := types.Message{
		Role: "user",
		Parts: []types.Part{{
			Type: types.PartTypeText,
			Text: "Test message",
		}},
	}
	success := store.AddMessageToHistory("task1", msg)
	assert.True(t, success)

	task, _ := store.Get("task1")
	assert.Len(t, task.History, 1)
	assert.Equal(t, msg, task.History[0])

	// Test adding message to non-existent task
	success = store.AddMessageToHistory("nonexistent", msg)
	assert.False(t, success)
}

func TestTaskStore_SetPushNotification(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	store.Create("task1", "Test Task")

	// Test setting push notification
	config := types.PushNotificationConfig{
		URL: "http://example.com/webhook",
	}
	success := store.SetPushNotification("task1", config)
	assert.True(t, success)

	task, _ := store.Get("task1")
	assert.NotNil(t, task.PushNotification)
	assert.Equal(t, config.URL, task.PushNotification.URL)

	// Test setting push notification for non-existent task
	success = store.SetPushNotification("nonexistent", config)
	assert.False(t, success)
}

func TestTaskStore_GetHistory(t *testing.T) {
	store := NewInMemoryTaskStore()

	// Create a task
	store.Create("task1", "Test Task")

	// Add multiple messages
	messages := []types.Message{
		{
			Role:  "user",
			Parts: []types.Part{{Type: types.PartTypeText, Text: "Message 1"}},
		},
		{
			Role:  "assistant",
			Parts: []types.Part{{Type: types.PartTypeText, Text: "Message 2"}},
		},
		{
			Role:  "user",
			Parts: []types.Part{{Type: types.PartTypeText, Text: "Message 3"}},
		},
	}

	for _, msg := range messages {
		store.AddMessageToHistory("task1", msg)
	}

	// Test getting all history
	history := store.GetHistory("task1", 0)
	assert.Len(t, history, 3)
	assert.Equal(t, messages, history)

	// Test getting limited history
	limitedHistory := store.GetHistory("task1", 2)
	assert.Len(t, limitedHistory, 2)
	assert.Equal(t, messages[1:], limitedHistory)

	// Test getting history for non-existent task
	history = store.GetHistory("nonexistent", 0)
	assert.Nil(t, history)
}
