package v4

import (
	"context"
	"encoding/json"
	vmmConfig "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/prism/v4/config"
	"net/http"
	"net/http/httptest"
	"testing"

	vmmModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyEtagPreservesSourceAndWritesDestination(t *testing.T) {
	t.Parallel()
	source := &vmmModels.Vm{Reserved_: map[string]any{"eTAG": "current", "source": true}}
	destination := &vmmModels.Vm{Reserved_: map[string]any{"ETAG": "stale", "destination": true}}
	actual := CopyEtag(source, destination)
	require.Same(t, destination, actual)
	assert.Equal(t, map[string]any{"eTAG": "current", "source": true}, source.Reserved_)
	assert.Equal(t, map[string]any{"Etag": "current", "destination": true}, destination.Reserved_)
	shared := &vmmModels.Vm{Reserved_: source.Reserved_}
	require.Same(t, shared, CopyEtag(source, shared))
	shared.Reserved_["destination"] = true
	assert.Equal(t, map[string]any{"eTAG": "current", "source": true}, source.Reserved_)
	require.Same(t, source, CopyEtag(source, source))
	assert.Equal(t, "current", GetEtag(source))
	empty := &vmmModels.Vm{}
	require.Same(t, empty, CopyEtag(source, empty))
	assert.Equal(t, "current", GetEtag(empty))
	value := vmmModels.Vm{}
	copied, ok := CopyEtag(source, value).(vmmModels.Vm)
	require.True(t, ok)
	assert.Equal(t, "current", GetEtag(copied))
	assert.Nil(t, value.Reserved_)
}

func TestEtagInvalidShapes(t *testing.T) {
	t.Parallel()
	for _, input := range []any{
		nil, (*vmmModels.Vm)(nil), 1, new(int),
		struct{ Reserved_ string }{Reserved_: "invalid"},
		struct{ Reserved_ map[string]any }{Reserved_: map[string]any{"Etag": 1}},
	} {
		assert.NotPanics(t, func() {
			assert.Empty(t, GetEtag(input))
			assert.Equal(t, input, DropEtag(input))
			assert.Equal(t, input, CopyEtag(nil, input))
		})
	}
}

func TestDropEtagPreservesOtherMetadata(t *testing.T) {
	t.Parallel()
	input := &vmmModels.Vm{Reserved_: map[string]any{"Etag": "one", "eTAG": "two", "other": true}}
	require.Same(t, input, DropEtag(input))
	assert.Equal(t, map[string]any{"other": true}, input.Reserved_)
}

type unexpectedMetadataResponse struct{ Metadata any }

func (unexpectedMetadataResponse) GetData() any { return nil }

func TestMetadataInvalidShapes(t *testing.T) {
	t.Parallel()
	for _, input := range []APIResponse{
		(*fakeListResponse)(nil),
		unexpectedMetadataResponse{},
		unexpectedMetadataResponse{Metadata: 1},
		unexpectedMetadataResponse{Metadata: struct{ TotalAvailableResults string }{TotalAvailableResults: "wrong"}},
		unexpectedMetadataResponse{Metadata: &fakeListMetadata{}},
	} {
		assert.NotPanics(t, func() { assert.Zero(t, GetMetadataTotalResults(input)) })
	}
	total := 3
	assert.Equal(t, total, GetMetadataTotalResults(&fakeListResponse{Metadata: &fakeListMetadata{TotalAvailableResults: &total}}))
}

func TestVMUpdateAsyncCopiesEtag(t *testing.T) {
	t.Parallel()
	const etag = "current"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_, err := w.Write(getVmResponseBodyWithEtag(t, "vm", etag))
			assert.NoError(t, err)
			return
		}
		assert.Equal(t, http.MethodPut, request.Method)
		assert.Equal(t, etag, request.Header.Get("If-Match"))
		response := &vmmModels.UpdateVmApiResponse{}
		err := response.SetData(vmmConfig.TaskReference{ObjectType_: new("prism.v4.config.TaskReference"), ExtId: new("task")})
		assert.NoError(t, err)
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	t.Cleanup(server.Close)
	service := newVMServiceAgainstServer(t, server)
	vm := &vmmModels.Vm{}
	operation, err := service.UpdateAsync(context.Background(), "vm", vm)
	require.NoError(t, err)
	require.NotNil(t, operation)
	assert.Equal(t, etag, GetEtag(vm))
}

func TestEtagConflictingAliasesFailClosed(t *testing.T) {
	t.Parallel()
	object := struct{ Reserved_ map[string]any }{Reserved_: map[string]any{"Etag": "first", "ETAG": "second"}}
	assert.Empty(t, GetEtag(object))
	_, _, err := GetEntityAndEtag(object, nil)
	require.Error(t, err)
}
