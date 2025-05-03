package state

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestNewManager(t *testing.T) {
	Convey("Given a state directory", t, func() {
		stateDir, err := os.MkdirTemp("", "state-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(stateDir)

		Convey("When creating a new manager", func() {
			manager, err := NewManager(stateDir)

			Convey("It should initialize successfully", func() {
				So(err, ShouldBeNil)
				So(manager, ShouldNotBeNil)
				So(manager.states, ShouldNotBeNil)
				So(len(manager.states), ShouldEqual, 0)
			})
		})
	})
}

func TestGetTask(t *testing.T) {
	Convey("Given a state manager with a task", t, func() {
		stateDir, err := os.MkdirTemp("", "state-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(stateDir)

		manager, err := NewManager(stateDir)
		So(err, ShouldBeNil)

		task := &types.Task{
			ID: "test-task",
			Status: types.TaskStatus{
				State: types.TaskStateSubmitted,
			},
		}
		manager.states[task.ID] = task

		Convey("When getting an existing task", func() {
			result, err := manager.GetTask(context.Background(), task.ID)

			Convey("It should return the task", func() {
				So(err, ShouldBeNil)
				So(result, ShouldEqual, task)
			})
		})

		Convey("When getting a non-existent task", func() {
			_, err := manager.GetTask(context.Background(), "nonexistent")

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestUpdateTask(t *testing.T) {
	Convey("Given a state manager", t, func() {
		stateDir, err := os.MkdirTemp("", "state-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(stateDir)

		manager, err := NewManager(stateDir)
		So(err, ShouldBeNil)

		task := &types.Task{
			ID: "test-task",
			Status: types.TaskStatus{
				State: types.TaskStateSubmitted,
			},
		}

		Convey("When updating a task", func() {
			err := manager.UpdateTask(context.Background(), task)

			Convey("It should update successfully", func() {
				So(err, ShouldBeNil)
				So(manager.states[task.ID], ShouldEqual, task)
			})

			Convey("When updating to an invalid state", func() {
				task.Status.State = types.TaskStateCompleted
				err := manager.UpdateTask(context.Background(), task)

				Convey("It should return an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
		})
	})
}

func TestRecoverTask(t *testing.T) {
	Convey("Given a state manager with a persisted task", t, func() {
		stateDir, err := os.MkdirTemp("", "state-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(stateDir)

		manager, err := NewManager(stateDir)
		So(err, ShouldBeNil)

		task := &types.Task{
			ID: "test-task",
			Status: types.TaskStatus{
				State: types.TaskStateSubmitted,
			},
		}

		// Persist task
		err = manager.UpdateTask(context.Background(), task)
		So(err, ShouldBeNil)

		// Create new manager
		manager, err = NewManager(stateDir)
		So(err, ShouldBeNil)

		Convey("When recovering the task", func() {
			err := manager.RecoverTask(context.Background(), task.ID)

			Convey("It should recover successfully", func() {
				So(err, ShouldBeNil)
				So(manager.states[task.ID], ShouldNotBeNil)
			})
		})
	})
}

func TestSubscribeToUpdates(t *testing.T) {
	Convey("Given a state manager", t, func() {
		stateDir, err := os.MkdirTemp("", "state-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(stateDir)

		manager, err := NewManager(stateDir)
		So(err, ShouldBeNil)

		Convey("When subscribing to updates", func() {
			updates := manager.SubscribeToUpdates(context.Background(), "test-task")

			Convey("When updating a task", func() {
				task := &types.Task{
					ID: "test-task",
					Status: types.TaskStatus{
						State: types.TaskStateSubmitted,
					},
				}

				err := manager.UpdateTask(context.Background(), task)
				So(err, ShouldBeNil)

				Convey("It should receive the update", func() {
					select {
					case update := <-updates:
						So(update, ShouldEqual, task)
					case <-time.After(time.Second):
						t.Fatal("timeout waiting for update")
					}
				})
			})
		})
	})
}

func TestCleanup(t *testing.T) {
	Convey("Given a state manager with old tasks", t, func() {
		stateDir, err := os.MkdirTemp("", "state-test-*")
		So(err, ShouldBeNil)
		defer os.RemoveAll(stateDir)

		manager, err := NewManager(stateDir)
		So(err, ShouldBeNil)

		// Create old task with timestamp
		oldTime := time.Now().Add(-2 * time.Hour)
		task := &types.Task{
			ID: "old-task",
			Status: types.TaskStatus{
				State:     types.TaskStateSubmitted,
				Timestamp: &oldTime,
			},
		}

		// Update task to create state file
		err = manager.UpdateTask(context.Background(), task)
		So(err, ShouldBeNil)

		Convey("When cleaning up old tasks", func() {
			err := manager.Cleanup(context.Background(), time.Hour)

			Convey("It should clean up successfully", func() {
				So(err, ShouldBeNil)
				So(manager.states[task.ID], ShouldBeNil)
			})
		})
	})
}
