package v4

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"
	"time"

	"k8s.io/utils/ptr"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v4prismGoClient "github.com/nutanix-cloud-native/prism-go-client/v4"

	gpuModels "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/ahv/config"
	clusterModels "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	dpModels "github.com/nutanix/ntnx-api-golang-clients/datapolicies-go-client/v4/models/datapolicies/v4/config"
	iamModels "github.com/nutanix/ntnx-api-golang-clients/iam-go-client/v4/models/iam/v4/authn"
	iamAuthzModels "github.com/nutanix/ntnx-api-golang-clients/iam-go-client/v4/models/iam/v4/authz"
	alertModels "github.com/nutanix/ntnx-api-golang-clients/monitoring-go-client/v4/models/monitoring/v4/serviceability"
	multidomainModels "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	subnetModels "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	networkingprismapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/prism/v4/config"
	prismModels "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	prismErrors "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/error"
	taskRequest "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/request/tasks"
	vmmModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
	policyModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/policies"
	contentModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	volumeModels "github.com/nutanix/ntnx-api-golang-clients/volumes-go-client/v4/models/volumes/v4/config"
)

// EntityReferenceGetterKey is the key for the entity reference getter in the context
const EntityReferenceGetterKey = "entityReferenceGetter"

// Client struct for converged client
// It contains implementation for all required API operations grouped by service
type Client struct {
	converged.Client[
		policyModels.VmAntiAffinityPolicy,
		clusterModels.Cluster,
		clusterModels.VirtualGpuProfile,
		clusterModels.PhysicalGpuProfile,
		gpuModels.VirtualGpuProfile,
		gpuModels.PhysicalGpuProfile,
		clusterModels.Host,
		prismModels.Category,
		contentModels.Image,
		contentModels.FileDetail,
		clusterModels.StorageContainer,
		subnetModels.Subnet,
		networkingprismapi.TaskReference,
		vmmModels.Vm,
		vmmModels.Nic,
		vmmModels.Disk,
		prismModels.Task,
		prismErrors.AppMessage,
		volumeModels.VolumeGroup,
		volumeModels.VmAttachment,
		vmmModels.VmProfile,
		vmmModels.DeployVmFromVmProfileParams,
		prismModels.DomainManager,
		iamModels.User,
		iamAuthzModels.Role,
		iamAuthzModels.RoleMembership,
		iamAuthzModels.RoleMembershipProjection,
		iamAuthzModels.AuthorizationPolicy,
		iamAuthzModels.AuthorizationPolicyProjection,
		iamAuthzModels.Operation,
		contentModels.Template,
		contentModels.TemplateDeployment,
		contentModels.Ova,
		contentModels.FileDetail,
		dpModels.ProtectionPolicy,
		dpModels.RecoveryPlan,
		multidomainModels.Project,
		multidomainModels.ResourceGroup,
		clusterModels.Disk,
		alertModels.Alert,
	]

	client *v4prismGoClient.Client
}

// NewClient creates a new converged client
func NewClient(credentials prismgoclient.Credentials, opts ...types.ClientOption[v4prismGoClient.Client]) (*Client, error) {
	v4sdkClient, err := v4prismGoClient.NewV4Client(credentials, opts...)
	if err != nil {
		return nil, err
	}
	return NewClientFromV4SDKClient(v4sdkClient), nil
}

// NewClientFromV4SDKClient creates a new converged client from a V4 SDK client
// It initializes the V4 client and creates the service implementations
func NewClientFromV4SDKClient(v4sdkClient *v4prismGoClient.Client) *Client {
	return &Client{
		Client: converged.Client[
			policyModels.VmAntiAffinityPolicy,
			clusterModels.Cluster,
			clusterModels.VirtualGpuProfile,
			clusterModels.PhysicalGpuProfile,
			gpuModels.VirtualGpuProfile,
			gpuModels.PhysicalGpuProfile,
			clusterModels.Host,
			prismModels.Category,
			contentModels.Image,
			contentModels.FileDetail,
			clusterModels.StorageContainer,
			subnetModels.Subnet,
			networkingprismapi.TaskReference,
			vmmModels.Vm,
			vmmModels.Nic,
			vmmModels.Disk,
			prismModels.Task,
			prismErrors.AppMessage,
			volumeModels.VolumeGroup,
			volumeModels.VmAttachment,
			vmmModels.VmProfile,
			vmmModels.DeployVmFromVmProfileParams,
			prismModels.DomainManager,
			iamModels.User,
			iamAuthzModels.Role,
			iamAuthzModels.RoleMembership,
			iamAuthzModels.RoleMembershipProjection,
			iamAuthzModels.AuthorizationPolicy,
			iamAuthzModels.AuthorizationPolicyProjection,
			iamAuthzModels.Operation,
			contentModels.Template,
			contentModels.TemplateDeployment,
			contentModels.Ova,
			contentModels.FileDetail,
			dpModels.ProtectionPolicy,
			dpModels.RecoveryPlan,
			multidomainModels.Project,
			multidomainModels.ResourceGroup,
			clusterModels.Disk,
			alertModels.Alert,
		]{
			AntiAffinityPolicies:  NewAntiAffinityPoliciesService(v4sdkClient),
			Clusters:              NewClustersService(v4sdkClient),
			Categories:            NewCategoriesService(v4sdkClient),
			Images:                NewImagesService(v4sdkClient),
			StorageContainers:     NewStorageContainersService(v4sdkClient),
			Subnets:               NewSubnetsService(v4sdkClient),
			VMs:                   NewVMsService(v4sdkClient),
			Tasks:                 NewTasksService(v4sdkClient),
			VolumeGroups:          NewVolumeGroupsService(v4sdkClient),
			VMProfiles:            NewVMProfilesService(v4sdkClient),
			DomainManager:         NewDomainManagerService(v4sdkClient),
			Users:                 NewUsersService(v4sdkClient),
			Roles:                 NewRolesService(v4sdkClient),
			RoleMemberships:       NewRoleMembershipsService(v4sdkClient),
			AuthorizationPolicies: NewAuthorizationPoliciesService(v4sdkClient),
			Operations:            NewOperationsService(v4sdkClient),
			Templates:             NewTemplatesService(v4sdkClient),
			Ovas:                  NewOvasService(v4sdkClient),
			DataPolicies: converged.DataPolicies[dpModels.ProtectionPolicy, dpModels.RecoveryPlan]{
				ProtectionPolicies: NewProtectionPoliciesService(v4sdkClient),
				RecoveryPlans:      NewRecoveryPlansService(v4sdkClient),
			},
			Projects:       NewProjectsService(v4sdkClient),
			ResourceGroups: NewResourceGroupsService(v4sdkClient),
			Disks:          NewDisksService(v4sdkClient),
			Alerts:         NewAlertsService(v4sdkClient),
		},
		client: v4sdkClient,
	}
}

// Operation contains a PC task and its locally observed results.
// Construct operations with NewOperation. Copies share synchronized state.
type Operation[T any] struct {
	state        *operationState[T]
	taskUUID     string
	getTask      func(context.Context) (prismModels.Task, error)
	entityGetter func(context.Context, string) (*T, error)
}

type operationState[T any] struct {
	lock               sync.Mutex
	affectedEntityRefs []prismModels.EntityReference
	entityUUIDs        []string
	taskStatus         converged.TaskStatus
	taskErrors         []error
	result             []*T
	run                *operationRun[T]
	finished           bool
}

type operationRun[T any] struct {
	done   chan struct{}
	result []*T
	err    error
}

// NewOperation creates a new Operation for the given PC async task UUID and client.
func NewOperation[T any](
	taskUUID string,
	client *v4prismGoClient.Client,
	entityGetter func(ctx context.Context, uuid string) (*T, error)) *Operation[T] {
	return &Operation[T]{
		state:    &operationState[T]{taskStatus: converged.TaskStatusQueued, taskErrors: make([]error, 0)},
		taskUUID: taskUUID,
		getTask: func(ctx context.Context) (prismModels.Task, error) {
			if client == nil || client.TasksApiInstance == nil || client.TasksApiInstance.ServiceClient == nil {
				return prismModels.Task{}, errors.New("task client is not initialized")
			}
			return CallAPI[*prismModels.GetTaskApiResponse, prismModels.Task](
				client.TasksApiInstance.ServiceClient.GetTaskById(ctx, &taskRequest.GetTaskByIdRequest{ExtId: &taskUUID}),
			)
		},
		entityGetter: entityGetter,
	}
}

// Wait shares one poller and result fetch among concurrent callers. A waiting
// caller can cancel independently; the poller's context controls its own attempt.
// Local cancellation never cancels or deletes the remote task or its entities.
// Terminal outcomes are reused. An interrupted attempt can be resumed by Wait.
func (o *Operation[T]) Wait(ctx context.Context) ([]*T, error) {
	if o.state == nil || o.getTask == nil {
		return nil, errors.New("operation is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("task wait canceled: %w", err)
	}
	o.state.lock.Lock()
	if o.state.finished {
		run := o.state.run
		o.state.lock.Unlock()
		return slices.Clone(run.result), run.err
	}
	if o.state.run != nil {
		run := o.state.run
		o.state.lock.Unlock()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("task wait canceled: %w", ctx.Err())
		case <-run.done:
			return slices.Clone(run.result), run.err
		}
	}
	run := &operationRun[T]{done: make(chan struct{})}
	o.state.run = run
	o.state.lock.Unlock()

	run.result, run.err = o.wait(ctx)
	o.state.lock.Lock()
	// Retry local interruption and lookup failures, but never refetch a completed outcome.
	o.state.finished = run.err == nil || o.state.taskStatus == converged.TaskStatusFailed || o.state.taskStatus == converged.TaskStatusCanceled ||
		(o.state.taskStatus == converged.TaskStatusSucceeded && !errors.Is(run.err, context.Canceled) && !errors.Is(run.err, context.DeadlineExceeded))
	if !o.state.finished {
		o.state.run = nil
	}
	close(run.done)
	o.state.lock.Unlock()
	return slices.Clone(run.result), run.err
}

func (o *Operation[T]) wait(ctx context.Context) ([]*T, error) {
	for o.Status() != converged.TaskStatusSucceeded {
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("task wait canceled: %w", ctx.Err())
		case <-timer.C:
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("task wait canceled: %w", err)
		}
		task, err := o.getTask(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get task %s: %w", o.taskUUID, err)
		}
		if task.Status == nil {
			return nil, fmt.Errorf("task %s status is nil", o.taskUUID)
		}
		o.state.lock.Lock()
		o.state.taskStatus = ConvertTaskStatus(*task.Status)
		switch *task.Status {
		case prismModels.TASKSTATUS_FAILED, prismModels.TASKSTATUS_CANCELED:
			for _, msg := range task.ErrorMessages {
				if msg.Message != nil {
					o.state.taskErrors = append(o.state.taskErrors, fmt.Errorf("task %s error: %s", o.taskUUID, *msg.Message))
				}
			}
			err := fmt.Errorf("task %s status %s: %w", o.taskUUID, o.state.taskStatus, converged.ErrTaskFailed)
			if len(o.state.taskErrors) == 0 {
				o.state.taskErrors = append(o.state.taskErrors, err)
			} else {
				err = errors.Join(err, errors.Join(o.state.taskErrors...))
			}
			o.state.lock.Unlock()
			return nil, err
		}
		if task.EntitiesAffected != nil {
			o.state.affectedEntityRefs = slices.Clone(task.EntitiesAffected)
		}
		for _, ref := range task.EntitiesAffected {
			if ref.ExtId == nil {
				o.state.lock.Unlock()
				return nil, fmt.Errorf("task %s affected entity reference is nil or has no UUID", o.taskUUID)
			}
			// Duplicate references have been observed in VM creation responses.
			if !slices.Contains(o.state.entityUUIDs, *ref.ExtId) {
				o.state.entityUUIDs = append(o.state.entityUUIDs, *ref.ExtId)
			}
		}
		o.state.lock.Unlock()
	}

	result := make([]*T, 0)
	for _, uuid := range o.state.entityUUIDs {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("task wait canceled: %w", err)
		}
		if o.entityGetter == nil {
			return nil, errors.New("entity getter is not initialized")
		}
		entity, err := o.entityGetter(ctx, uuid)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("task wait canceled: %w", ctx.Err())
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("failed to get entity %s: %w", uuid, err)
		}
		// Some VM tasks include unrelated, missing entities; delete getters return nil, nil.
		if entity != nil {
			if err != nil {
				return nil, fmt.Errorf("failed to get entity %s: %w", uuid, err)
			}
			result = append(result, entity)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("task wait canceled: %w", err)
	}
	o.state.lock.Lock()
	o.state.result = result
	o.state.lock.Unlock()
	return result, nil
}

// Results returns a separate result slice. Entity pointers remain caller-owned
// and must not be mutated concurrently with other readers.
func (o *Operation[T]) Results() ([]*T, error) {
	if o.state == nil {
		return nil, converged.ErrTaskNotComplete
	}
	o.state.lock.Lock()
	defer o.state.lock.Unlock()
	if o.state.taskStatus == converged.TaskStatusFailed || o.state.taskStatus == converged.TaskStatusCanceled {
		return nil, converged.ErrTaskFailed
	}
	if o.state.taskStatus != converged.TaskStatusSucceeded {
		return nil, converged.ErrTaskNotComplete
	}
	if o.state.result == nil {
		return nil, converged.ErrTaskResultsNotReady
	}
	return slices.Clone(o.state.result), nil
}

// UUID returns the UUID of the PC async task.
func (o *Operation[T]) UUID() string { return o.taskUUID }

// Status returns the last observed status of the PC async task.
func (o *Operation[T]) Status() converged.TaskStatus {
	if o.state == nil {
		return ""
	}
	o.state.lock.Lock()
	defer o.state.lock.Unlock()
	return o.state.taskStatus
}

// Errors returns a separate slice of observed task errors.
func (o *Operation[T]) Errors() []error {
	if o.state == nil {
		return nil
	}
	o.state.lock.Lock()
	defer o.state.lock.Unlock()
	return slices.Clone(o.state.taskErrors)
}

// GetAffectedEntityRefs returns independent copies of generated references,
// including their pointer fields and JSON extension metadata.
func (o *Operation[T]) GetAffectedEntityRefs() ([]any, error) {
	if o.state == nil {
		return []any{}, nil
	}
	o.state.lock.Lock()
	defer o.state.lock.Unlock()
	data, err := json.Marshal(o.state.affectedEntityRefs)
	if err != nil {
		return nil, fmt.Errorf("failed to copy affected entity references: %w", err)
	}
	var refs []prismModels.EntityReference
	if err := json.Unmarshal(data, &refs); err != nil {
		return nil, fmt.Errorf("failed to copy affected entity references: %w", err)
	}
	result := make([]any, len(refs))
	for i := range refs {
		result[i] = &refs[i]
	}
	return result, nil
}

// IsDone checks whether the remote task has reached a terminal state.
func (o *Operation[T]) IsDone() bool {
	status := o.Status()
	return status == converged.TaskStatusSucceeded || status == converged.TaskStatusFailed || status == converged.TaskStatusCanceled
}

// IsSuccess checks if the PC async task is successful.
func (o *Operation[T]) IsSuccess() bool { return o.Status() == converged.TaskStatusSucceeded }

// IsFailed checks if the PC async task is failed.
func (o *Operation[T]) IsFailed() bool { return o.Status() == converged.TaskStatusFailed }

// NewIterator creates a new iterator for the given list function and options.
func NewIterator[R APIResponse, T any](
	ctx context.Context,
	listFunc func(context.Context, *V4ODataParams) (R, error),
	opts ...converged.ODataOption,
) converged.Iterator[T] {
	var zero T

	iterFunc := func(yield func(T, error) bool) {
		var items []T
		var err error

		iterateAllPages := false
		totalCount := 0
		batchIndex := 0
		totalIndex := 0

		reqParams, err := OptsToV4ODataParams(opts...)
		if err != nil {
			yield(zero, err)
			return
		}

		page := 0
		if reqParams.Limit != nil && reqParams.Page == nil {
			reqParams.Page = ptr.To(page)
		}

		if reqParams.Page == nil {
			reqParams.Page = ptr.To(page) // Start from the first page
			reqParams.Limit = nil         // Let API use the default limit
			iterateAllPages = true        // Iterate through all pages
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// continue with the loop
			}

			// Get next page
			items, totalCount, err = CallListAPI[R, T](listFunc(ctx, reqParams))
			if err != nil {
				yield(zero, err)
				return
			}

			for batchIndex < len(items) && totalIndex < totalCount {
				select {
				case <-ctx.Done():
					return
				default:
					// continue with the loop
				}

				if !yield(items[batchIndex], nil) {
					return
				}

				totalIndex += 1
				batchIndex += 1
			}

			if totalIndex >= totalCount || !iterateAllPages || len(items) == 0 {
				return // All items have been yielded or we are not iterating through all pages
			}

			reqParams.Page = ptr.To(*reqParams.Page + 1) // Move to the next page
			batchIndex = 0
		}
	}

	return converged.Iterator[T](iter.Seq2[T, error](iterFunc))
}

// GetPrismEntityReference try to convert the entity reference to the prism entity reference.
func GetPrismEntityReference(entityRef any) (*prismModels.EntityReference, error) {
	entityRefValue, ok := entityRef.(*prismModels.EntityReference)
	if !ok {
		return nil, fmt.Errorf("entity reference is not a *prismModels.EntityReference")
	}
	return entityRefValue, nil
}

// GetPrismEntityReferenceSlice try to convert the entity reference slice to the prism entity reference slice.
func GetPrismEntityReferenceSlice(entityRefs []any) ([]*prismModels.EntityReference, error) {
	entityRefsValue := make([]*prismModels.EntityReference, len(entityRefs))
	for i, entityRef := range entityRefs {
		entityRefValue, err := GetPrismEntityReference(entityRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get prism entity reference: %w", err)
		}
		entityRefsValue[i] = entityRefValue
	}
	return entityRefsValue, nil
}
