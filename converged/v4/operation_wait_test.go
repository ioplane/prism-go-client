package v4

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nutanix-cloud-native/prism-go-client/converged"
	prismModels "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	prismErrors "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func fakeTaskOperation[T any](task func(context.Context) (prismModels.Task, error), getter func(context.Context, string) (*T, error)) *Operation[T] {
	o := NewOperation("task-id", nil, getter)
	o.getTask = task
	return o
}

func TestOperationWaitTerminal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   prismModels.TaskStatus
		messages []prismErrors.AppMessage
	}{
		{"failed-nil", prismModels.TASKSTATUS_FAILED, nil},
		{"failed-empty", prismModels.TASKSTATUS_FAILED, []prismErrors.AppMessage{}},
		{"failed-nil-message", prismModels.TASKSTATUS_FAILED, []prismErrors.AppMessage{{}}},
		{"failed-message", prismModels.TASKSTATUS_FAILED, []prismErrors.AppMessage{{Message: ptr.To("failure")}}},
		{"canceled", prismModels.TASKSTATUS_CANCELED, nil},
		{"succeeded", prismModels.TASKSTATUS_SUCCEEDED, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				o := fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
					calls++
					return prismModels.Task{Status: &tc.status, ErrorMessages: tc.messages}, nil
				}, converged.NoEntityGetter)
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancel()
				result, err := o.Wait(ctx)
				if tc.status == prismModels.TASKSTATUS_SUCCEEDED {
					require.NoError(t, err)
					require.Empty(t, result)
					ready, resultErr := o.Results()
					require.NoError(t, resultErr)
					require.Empty(t, ready)
				} else {
					require.ErrorIs(t, err, converged.ErrTaskFailed)
					require.NotErrorIs(t, err, context.DeadlineExceeded)
					_, resultErr := o.Results()
					require.ErrorIs(t, resultErr, converged.ErrTaskFailed)
					taskErrors := o.Errors()
					require.NotEmpty(t, taskErrors)
					taskErrors[0] = nil
					require.NotNil(t, o.Errors()[0])
				}
				require.True(t, o.IsDone())
				require.Equal(t, ConvertTaskStatus(tc.status), o.Status())
				_, repeatErr := o.Wait(context.Background())
				require.Equal(t, err, repeatErr)
				require.Equal(t, 1, calls, "terminal repeats must not poll again")
			})
		})
	}
}

func TestOperationWaitCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "deadline"}[deadline], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				o := fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
					calls++
					return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_RUNNING)}, nil
				}, converged.NoEntityGetter)
				var ctx context.Context
				var cancel context.CancelFunc
				wantErr := context.Canceled
				if deadline {
					ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
					wantErr = context.DeadlineExceeded
				} else {
					ctx, cancel = context.WithCancel(context.Background())
				}
				defer cancel()
				start := time.Now()
				done := make(chan error, 1)
				go func() { _, err := o.Wait(ctx); done <- err }()
				synctest.Wait()
				if !deadline {
					cancel()
				}
				require.ErrorIs(t, <-done, wantErr)
				require.Less(t, time.Since(start), time.Second, "cancellation must interrupt the poll timer")
				require.Zero(t, calls, "canceled timer must not start a task request")
				o.getTask = func(context.Context) (prismModels.Task, error) {
					calls++
					return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED)}, nil
				}
				_, err := o.Wait(context.Background())
				require.NoError(t, err)
				require.Equal(t, 1, calls)
			})
		})
	}
}

func TestOperationPendingStates(t *testing.T) {
	for _, status := range []prismModels.TaskStatus{prismModels.TASKSTATUS_QUEUED, prismModels.TASKSTATUS_RUNNING, prismModels.TASKSTATUS_SUSPENDED, prismModels.TASKSTATUS_CANCELING, prismModels.TASKSTATUS_UNKNOWN, prismModels.TASKSTATUS_REDACTED} {
		t.Run(string(ConvertTaskStatus(status)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				o := fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
					calls++
					if calls == 1 {
						return prismModels.Task{Status: &status}, nil
					}
					return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED)}, nil
				}, converged.NoEntityGetter)
				done := make(chan struct{})
				go func() { defer close(done); _, err := o.Wait(context.Background()); assert.NoError(t, err) }()
				time.Sleep(time.Second)
				synctest.Wait()
				assert.Equal(t, ConvertTaskStatus(status), o.Status())
				assert.False(t, o.IsDone())
				_, err := o.Results()
				assert.ErrorIs(t, err, converged.ErrTaskNotComplete)
				<-done
				require.Equal(t, 2, calls)
			})
		})
	}
}

func TestOperationWaitEntitiesAndCopies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls, gets := 0, 0
		o := fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
			calls++
			return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED), EntitiesAffected: []prismModels.EntityReference{{ExtId: ptr.To("one"), UnknownFields_: map[string]interface{}{"nested": map[string]interface{}{"value": "original"}}}, {ExtId: ptr.To("one")}, {ExtId: ptr.To("gone")}}}, nil
		}, func(_ context.Context, uuid string) (*string, error) {
			gets++
			if uuid == "gone" {
				return nil, errors.New("best effort missing entity")
			}
			return &uuid, nil
		})
		result, err := o.Wait(context.Background())
		require.NoError(t, err)
		require.Len(t, result, 1)
		result[0] = nil
		ready, err := o.Results()
		require.NoError(t, err)
		require.Equal(t, "one", *ready[0])
		ready[0] = nil
		refs, err := o.GetAffectedEntityRefs()
		require.NoError(t, err)
		ref := refs[0].(*prismModels.EntityReference)
		*ref.ExtId = "changed"
		ref.UnknownFields_["nested"].(map[string]interface{})["value"] = "changed"
		refs, err = o.GetAffectedEntityRefs()
		require.NoError(t, err)
		ref = refs[0].(*prismModels.EntityReference)
		require.Equal(t, "one", *ref.ExtId)
		require.Equal(t, "original", ref.UnknownFields_["nested"].(map[string]interface{})["value"])
		copyOperation := *o
		result, err = copyOperation.Wait(context.Background())
		require.NoError(t, err)
		require.Equal(t, "one", *result[0])
		require.Equal(t, 1, calls)
		require.Equal(t, 2, gets)
	})
}

func TestOperationWaitConcurrent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls, gets atomic.Int32
		entered, release := make(chan struct{}), make(chan struct{})
		o := fakeTaskOperation(func(ctx context.Context) (prismModels.Task, error) {
			calls.Add(1)
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return prismModels.Task{}, ctx.Err()
			}
			return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED), EntitiesAffected: []prismModels.EntityReference{{ExtId: ptr.To("one")}}}, nil
		}, func(_ context.Context, uuid string) (*string, error) { gets.Add(1); return &uuid, nil })
		var wg sync.WaitGroup
		wg.Go(func() { _, err := o.Wait(context.Background()); assert.NoError(t, err) })
		<-entered
		ctx, cancel := context.WithCancel(context.Background())
		canceled := make(chan error, 1)
		go func() { _, err := o.Wait(ctx); canceled <- err }()
		synctest.Wait()
		cancel()
		require.ErrorIs(t, <-canceled, context.Canceled)
		for range 8 {
			wg.Go(func() { _, err := o.Wait(context.Background()); assert.NoError(t, err) })
		}
		wg.Go(func() {
			for range 128 {
				_ = o.UUID()
				_ = o.Status()
				_ = o.Errors()
				_, _ = o.Results()
				_, _ = o.GetAffectedEntityRefs()
				_ = o.IsDone()
				_ = o.IsSuccess()
				_ = o.IsFailed()
			}
		})
		synctest.Wait()
		close(release)
		wg.Wait()
		require.Equal(t, int32(1), calls.Load())
		require.Equal(t, int32(1), gets.Load())
	})
}

func TestOperationWaitEntityOutcomes(t *testing.T) {
	for _, outcome := range []string{"delete", "error", "cancel", "deadline"} {
		t.Run(outcome, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls, gets := 0, 0
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				failure := errors.New("entity failure")
				var o *Operation[converged.NoEntity]
				o = fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
					calls++
					return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED), EntitiesAffected: []prismModels.EntityReference{{ExtId: ptr.To("one")}}}, nil
				}, func(ctx context.Context, uuid string) (*converged.NoEntity, error) {
					gets++
					// Reentrant getters must not wait for a mutex held across callbacks.
					assert.Equal(t, converged.TaskStatusSucceeded, o.Status())
					if gets == 1 {
						switch outcome {
						case "error":
							return new(converged.NoEntity), failure
						case "cancel":
							cancel()
							return nil, ctx.Err()
						case "deadline":
							return nil, context.DeadlineExceeded
						}
					}
					return converged.NoEntityGetter(ctx, uuid)
				})
				_, err := o.Wait(ctx)
				switch outcome {
				case "delete":
					require.NoError(t, err)
				case "error":
					require.ErrorIs(t, err, failure)
				case "cancel":
					require.ErrorIs(t, err, context.Canceled)
				case "deadline":
					require.ErrorIs(t, err, context.DeadlineExceeded)
				}
				_, repeatErr := o.Wait(context.Background())
				if outcome == "error" {
					require.Equal(t, err, repeatErr)
					require.Equal(t, 1, gets)
					_, resultErr := o.Results()
					require.ErrorIs(t, resultErr, converged.ErrTaskResultsNotReady)
				} else {
					require.NoError(t, repeatErr)
					wantGets := 1
					if outcome != "delete" {
						wantGets = 2
					}
					require.Equal(t, wantGets, gets)
					_, resultErr := o.Results()
					require.NoError(t, resultErr)
				}
				require.Equal(t, 1, calls, "resume after entity interruption must not repoll a successful task")
			})
		})
	}
}

func TestOperationWaitOwnerCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan struct{})
		calls := 0
		o := fakeTaskOperation(func(ctx context.Context) (prismModels.Task, error) {
			calls++
			if calls == 1 {
				close(entered)
				<-ctx.Done()
				return prismModels.Task{}, ctx.Err()
			}
			return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED)}, nil
		}, converged.NoEntityGetter)
		owner, waiter := make(chan error, 1), make(chan error, 1)
		go func() { _, err := o.Wait(ctx); owner <- err }()
		<-entered
		go func() { _, err := o.Wait(context.Background()); waiter <- err }()
		synctest.Wait()
		cancel()
		require.ErrorIs(t, <-owner, context.Canceled)
		require.ErrorIs(t, <-waiter, context.Canceled)
		_, err := o.Wait(context.Background())
		require.NoError(t, err)
		require.Equal(t, 2, calls)
	})
}

func TestOperationWaitLookupErrorsAndZeroValue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var zero Operation[converged.NoEntity]
		require.Empty(t, zero.UUID())
		require.Empty(t, zero.Status())
		require.Empty(t, zero.Errors())
		require.False(t, zero.IsDone())
		require.False(t, zero.IsSuccess())
		require.False(t, zero.IsFailed())
		_, err := zero.Results()
		require.ErrorIs(t, err, converged.ErrTaskNotComplete)
		refs, err := zero.GetAffectedEntityRefs()
		require.NoError(t, err)
		require.Empty(t, refs)
		_, err = zero.Wait(context.Background())
		require.ErrorContains(t, err, "operation is not initialized")
		calls := 0
		failure := errors.New("lookup failure")
		o := fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
			calls++
			switch calls {
			case 1:
				return prismModels.Task{}, failure
			case 2:
				return prismModels.Task{}, nil
			default:
				return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED)}, nil
			}
		}, converged.NoEntityGetter)
		_, err = o.Wait(context.Background())
		require.ErrorIs(t, err, failure)
		_, err = o.Wait(context.Background())
		require.ErrorContains(t, err, "status is nil")
		_, err = o.Wait(context.Background())
		require.NoError(t, err)
		require.Equal(t, 3, calls)
		require.Equal(t, converged.TaskStatus("CANCELLING"), ConvertTaskStatus(prismModels.TASKSTATUS_CANCELING))
	})
}

func TestOperationWaitCanceledEmptyTask(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		o := fakeTaskOperation(func(context.Context) (prismModels.Task, error) {
			calls++
			if calls == 1 {
				cancel()
			}
			return prismModels.Task{Status: ptr.To(prismModels.TASKSTATUS_SUCCEEDED)}, nil
		}, converged.NoEntityGetter)
		_, err := o.Wait(ctx)
		require.ErrorIs(t, err, context.Canceled)
		_, err = o.Wait(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})
}
