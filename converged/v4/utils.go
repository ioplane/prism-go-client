package v4

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/nutanix-cloud-native/prism-go-client/converged"

	v4prismModels "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	"k8s.io/utils/ptr"
)

// V4ODataParams struct for V4 OData parameters
// It contains the OData parameters for the V4 API
type V4ODataParams struct {
	Page    *int
	Limit   *int
	Filter  *string
	OrderBy *string
	Expand  *string
	Select  *string
	Apply   *string
}

// ToV4ODataParams converts the OData options to V4 OData parameters
// It returns the V4 OData parameters if the options are valid, otherwise it returns an error
func ToV4ODataParams(params converged.ODataOptions) (*V4ODataParams, error) {
	if params == nil {
		return nil, nil
	}

	if v4Params, ok := params.(*V4ODataParams); ok {
		return v4Params, nil
	}

	return nil, fmt.Errorf("expected *V4ODataParams, got %T", params)
}

// SetPageOption sets the page option for the V4 OData parameters
// It returns an error if the page option is not valid
func (o *V4ODataParams) SetPageOption(page int) error {
	o.Page = &page
	return nil
}

// SetLimitOption sets the limit option for the V4 OData parameters
// It returns an error if the limit option is not valid
func (o *V4ODataParams) SetLimitOption(limit int) error {
	o.Limit = &limit
	return nil
}

// SetFilterOption sets the filter option for the V4 OData parameters
// It returns an error if the filter option is not valid
func (o *V4ODataParams) SetFilterOption(filter string) error {
	o.Filter = &filter
	return nil
}

// SetOrderByOption sets the order by option for the V4 OData parameters
// It returns an error if the order by option is not valid
func (o *V4ODataParams) SetOrderByOption(orderBy string) error {
	o.OrderBy = &orderBy
	return nil
}

// SetExpandOption sets the expand option for the V4 OData parameters
// It returns an error if the expand option is not valid
func (o *V4ODataParams) SetExpandOption(expand string) error {
	o.Expand = &expand
	return nil
}

// SetSelectOption sets the select option for the V4 OData parameters
// It returns an error if the select option is not valid
func (o *V4ODataParams) SetSelectOption(selectFields string) error {
	o.Select = &selectFields
	return nil
}

// SetApplyOption sets the apply option for the V4 OData parameters
// It returns an error if the apply option is not valid
func (o *V4ODataParams) SetApplyOption(apply string) error {
	o.Apply = &apply
	return nil
}

// structField reads an exported field from a struct or its non-nil pointer.
func structField(object any, name string) reflect.Value {
	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanInterface() {
		return reflect.Value{}
	}
	return field
}

func reservedMetadata(object any) map[string]any {
	field := structField(object, "Reserved_")
	if !field.IsValid() {
		return nil
	}
	metadata, ok := field.Interface().(map[string]any)
	if !ok {
		return nil
	}
	return metadata
}

// GetEtag returns the string ETag, or an empty string for absent, invalid or conflicting metadata.
func GetEtag(object any) string {
	var etag string
	found := false
	for key, value := range reservedMetadata(object) {
		if !strings.EqualFold(key, "etag") {
			continue
		}
		token, ok := value.(string)
		if !ok || (found && token != etag) {
			return ""
		}
		etag, found = token, true
	}
	return etag
}

// DropEtag removes ETag keys and returns the original object.
func DropEtag(object any) any {
	metadata := reservedMetadata(object)
	for key := range metadata {
		if strings.EqualFold(key, "etag") {
			delete(metadata, key)
		}
	}
	return object
}

// CopyEtag copies the ETag without mutating metadata shared with the source.
// Pointer destinations retain their identity; struct values return an updated copy.
func CopyEtag(source, destination any) any {
	value := reflect.ValueOf(destination)
	if value.Kind() == reflect.Struct {
		copied := reflect.New(value.Type()).Elem()
		copied.Set(value)
		value = copied
	} else if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return destination
	}
	field := value.FieldByName("Reserved_")
	if !field.IsValid() || !field.CanSet() || field.Type() != reflect.TypeFor[map[string]any]() {
		return destination
	}
	etag := GetEtag(source)
	metadata := maps.Clone(reservedMetadata(destination))
	for key := range metadata {
		if strings.EqualFold(key, "etag") {
			delete(metadata, key)
		}
	}
	if etag != "" {
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata["Etag"] = etag
	}
	field.Set(reflect.ValueOf(metadata))
	if reflect.TypeOf(destination).Kind() == reflect.Struct {
		return value.Interface()
	}
	return destination
}

var (
	V4TaskStatusesMap = map[v4prismModels.TaskStatus]converged.TaskStatus{
		v4prismModels.TASKSTATUS_CANCELED:  converged.TaskStatusCanceled,
		v4prismModels.TASKSTATUS_CANCELING: converged.TaskStatusCanceling,
		v4prismModels.TASKSTATUS_FAILED:    converged.TaskStatusFailed,
		v4prismModels.TASKSTATUS_QUEUED:    converged.TaskStatusQueued,
		v4prismModels.TASKSTATUS_RUNNING:   converged.TaskStatusRunning,
		v4prismModels.TASKSTATUS_SUSPENDED: converged.TaskStatusSuspended,
		v4prismModels.TASKSTATUS_SUCCEEDED: converged.TaskStatusSucceeded,
		v4prismModels.TASKSTATUS_UNKNOWN:   converged.TaskStatusUnknown,
		v4prismModels.TASKSTATUS_REDACTED:  converged.TaskStatusRedacted,
	}
)

// ConvertTaskStatus converts the V4 task status to the converged task status
// It returns the converted task status if the status is valid, otherwise it returns the unknown task status
func ConvertTaskStatus(status v4prismModels.TaskStatus) converged.TaskStatus {
	if convertedStatus, ok := V4TaskStatusesMap[status]; ok {
		return convertedStatus
	}
	return converged.TaskStatusUnknown
}

// APIResponse interface for the API response
// It contains the method to get the data from the API response
type APIResponse interface {
	GetData() any
}

// CallAPI calls the API and returns the result
// It returns the result if the API call is successful, otherwise it returns an error
func CallAPI[R APIResponse, T any](response R, err error) (T, error) {
	var zero, result T
	if err != nil {
		return zero, CategoriseFromOpenAPI(err)
	}

	data := response.GetData()
	if data == nil {
		return zero, nil
	}

	result, ok := data.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected type for API response data: %T", data)
	}

	return result, nil
}

// GetMetadataTotalResults returns the nonnegative total, or zero for absent or invalid metadata.
func GetMetadataTotalResults[R APIResponse](response R) int {
	metadata := structField(response, "Metadata")
	if !metadata.IsValid() {
		return 0
	}
	total := structField(metadata.Interface(), "TotalAvailableResults")
	if !total.IsValid() {
		return 0
	}
	count, ok := total.Interface().(*int)
	if !ok || count == nil || *count < 0 {
		return 0
	}
	return *count
}

// CallListAPI calls the list API and returns the result
// It returns the result if the API call is successful, otherwise it returns an error
func CallListAPI[R APIResponse, T any](response R, err error) ([]T, int, error) {
    var zero []T
    if err != nil {
        return zero, 0, CategoriseFromOpenAPI(err)
    }

    totalCount := GetMetadataTotalResults(response)

    data := response.GetData()
    if data == nil {
        return zero, totalCount, nil
    }

    result, ok := data.([]T)
    if !ok {
        // List responses can come back as either the full entity type or a
        // *Projection type. Projections are used when the API supports $expand
        // (for example AuthorizationPolicyProjection can include the related Role).
        // We must not convert a non-empty projection into []T, because that would
        // drop the nested expanded data the caller asked for.
        //
        // Empty lists are a special case: the generated SDK sometimes labels an
        // empty result as a projection type even when the caller expected the
        // full entity type. There is nothing in an empty list to lose, so treat
        // that as "no results" and return an empty []T.
        if v := reflect.ValueOf(data); v.Kind() == reflect.Slice && v.Len() == 0 {
            return zero, totalCount, nil
        }
        return zero, 0, fmt.Errorf("unexpected type for API response data: %T", data)
    }

    return result, totalCount, nil
}

// GetEntityAndEtag gets the entity and etag from the API response
// It returns the entity and etag if the API call is successful, otherwise it returns an error
func GetEntityAndEtag[T any](entity T, err error) (T, map[string]any, error) {
	var zero T

	if err != nil {
		return zero, nil, CategoriseFromOpenAPI(err)
	}

	etag := GetEtag(entity)
	if etag == "" {
		return zero, nil, fmt.Errorf("no ETag found for entity of type %T", entity)
	}

	args := map[string]any{
		"If-Match": &etag,
	}

	return entity, args, nil
}

// requestIDHeader is the header V4 APIs use to make write operations idempotent.
const requestIDHeader = "NTNX-Request-Id"

// headersContextKey is an unexported context key type to avoid collisions.
type headersContextKey struct{}

func withHeader(ctx context.Context, key, value string) context.Context {
	headers := maps.Clone(headersFromContext(ctx))
	if headers == nil {
		headers = make(map[string]string, 1)
	}
	headers[key] = value
	return context.WithValue(ctx, headersContextKey{}, headers)
}

// WithRequestID returns a copy of ctx carrying the given idempotency request id.
// Currently scoped for Create() operations. Callers that do not set it are unaffected 
// and no header is sent.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return withHeader(ctx, requestIDHeader, requestID)
}

// headersFromContext returns the headers set via withHeader, or nil if none.
func headersFromContext(ctx context.Context) map[string]string {
	headers, _ := ctx.Value(headersContextKey{}).(map[string]string)
	return headers
}

// headerArgs builds the SDK args (header map) from headers set on the context.
func headerArgs(ctx context.Context) map[string]any {
	headers := headersFromContext(ctx)
	if len(headers) == 0 {
		return nil
	}

	args := make(map[string]any, len(headers))
	for k, v := range headers {
		args[k] = ptr.To(v)
	}
	return args
}

// OptsToV4ODataParams converts the OData options to V4 OData parameters
// It returns the V4 OData parameters if the options are valid, otherwise it returns an error
func OptsToV4ODataParams(opts ...converged.ODataOption) (*V4ODataParams, error) {
	params := &V4ODataParams{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(params); err != nil {
				return nil, fmt.Errorf("failed to apply OData option: %w", err)
			}
		}
	}
	return params, nil
}

// Generic implementation of the Getter interface
// It returns the entity if the API call is successful, otherwise it returns an error
func GenericGetEntity[R APIResponse, T any](apiCall func() (R, error), entityName string) (*T, error) {
	result, err := CallAPI[R, T](apiCall())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", entityName, err)
	}
	return &result, nil
}

// GenericListEntities lists the entities
// It returns the entities if the API call is successful, otherwise it returns an error
func GenericListEntities[R APIResponse, T any](apiCall func(reqParams *V4ODataParams) (R, error), options []converged.ODataOption, entitiesName string) ([]T, error) {
	returnAll := false
	page := 0

	reqParams, err := OptsToV4ODataParams(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to convert options to V4ODataParams: %w", err)
	}

	if reqParams.Limit != nil && reqParams.Page == nil {
		reqParams.Page = ptr.To(page)
	}

	if reqParams.Page == nil {
		returnAll = true
		reqParams.Page = ptr.To(page)
		reqParams.Limit = nil // Let API use the default limit
	}

	result := []T{}

	items, totalCount, err := CallListAPI[R, T](apiCall(reqParams))
	if err != nil {
		return nil, fmt.Errorf("failed to list all %s: %w", entitiesName, err)
	}
	result = append(result, items...)

	if returnAll {
		for len(result) < totalCount {
			page++
			reqParams.Page = ptr.To(page)
			moreItems, latestTotalCount, err := CallListAPI[R, T](apiCall(reqParams))
			if err != nil {
				return nil, fmt.Errorf("failed to list all %s on page %d: %w", entitiesName, *reqParams.Page, err)
			}
			// Guard against an unbounded loop. metadata.TotalAvailableResults
			// and the page data are only eventually consistent, so the data set
			// can shrink between requests (entities being deleted) and the count
			// can lag behind it. If we ever get a page with no items, the server
			// has nothing more to return, so stop instead of paging forever past
			// the last page of data.
			if len(moreItems) == 0 {
				break
			}
			result = append(result, moreItems...)
			// Re-read the total from each page so that entities added while we
			// paginate are still collected, rather than stopping at the (now
			// stale) total observed on the first page.
			totalCount = latestTotalCount
		}
	}

	return result, nil
}

// ErrorOnlyIterator returns an iterator that yields exactly one (zero, err) so the first iteration gets the error.
func ErrorOnlyIterator[T any](err error) converged.Iterator[T] {
	var zero T
	seq := func(yield func(T, error) bool) {
		yield(zero, err)
	}
	return converged.Iterator[T](seq)
}

// GenericNewIterator creates a new iterator for the given API call and options
// It returns the iterator if the API call is successful, otherwise it returns an error
func GenericNewIterator[R APIResponse, T any](ctx context.Context, apiCall func(ctx context.Context, reqParams *V4ODataParams) (R, error), options []converged.ODataOption, entitiesName string) converged.Iterator[T] {
	return NewIterator[R, T](
		ctx,
		func(ctx context.Context, reqParams *V4ODataParams) (R, error) {
			return apiCall(ctx, reqParams)
		},
		options...,
	)
}
