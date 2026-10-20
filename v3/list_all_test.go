package v3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

func TestListAllPagination(t *testing.T) {
	operations := []struct {
		name, path, kind string
		list             func(context.Context, Operations) (any, error)
	}{
		{"ListAllVM", "/vms/list", "vm", func(ctx context.Context, op Operations) (any, error) { return op.ListAllVM(ctx, "name==keep") }},
		{"ListAllSubnet", "/subnets/list", "subnet", func(ctx context.Context, op Operations) (any, error) { return op.ListAllSubnet(ctx, "name==keep", nil) }},
		{"ListAllNetworkSecurityRule", "/network_security_rules/list", "network_security_rule", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllNetworkSecurityRule(ctx, "name==keep")
		}},
		{"ListAllImage", "/images/list", "image", func(ctx context.Context, op Operations) (any, error) { return op.ListAllImage(ctx, "name==keep") }},
		{"ListAllCluster", "/clusters/list", "cluster", func(ctx context.Context, op Operations) (any, error) { return op.ListAllCluster(ctx, "name==keep") }},
		{"ListAllCategoryValues", "/categories/test/list", "category", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllCategoryValues(ctx, "test", "name==keep")
		}},
		{"ListAllProject", "/projects/list", "project", func(ctx context.Context, op Operations) (any, error) { return op.ListAllProject(ctx, "name==keep") }},
		{"ListAllAccessControlPolicy", "/access_control_policies/list", "access_control_policy", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllAccessControlPolicy(ctx, "name==keep")
		}},
		{"ListAllRole", "/roles/list", "role", func(ctx context.Context, op Operations) (any, error) { return op.ListAllRole(ctx, "name==keep") }},
		{"ListAllUser", "/users/list", "user", func(ctx context.Context, op Operations) (any, error) { return op.ListAllUser(ctx, "name==keep") }},
		{"ListAllUserGroup", "/user_groups/list", "user_group", func(ctx context.Context, op Operations) (any, error) { return op.ListAllUserGroup(ctx, "name==keep") }},
		{"ListAllPermission", "/permissions/list", "permission", func(ctx context.Context, op Operations) (any, error) { return op.ListAllPermission(ctx, "name==keep") }},
		{"ListAllProtectionRules", "/protection_rules/list", "protection_rule", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllProtectionRules(ctx, "name==keep")
		}},
		{"ListAllRecoveryPlans", "/recovery_plans/list", "recovery_plan", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllRecoveryPlans(ctx, "name==keep")
		}},
		{"ListAllServiceGroups", "/service_groups/list", "service_group", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllServiceGroups(ctx, "name==keep")
		}},
		{"ListAllAddressGroups", "/address_groups/list", "address_group", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllAddressGroups(ctx, "name==keep")
		}},
		{"ListAllAvailabilityZones", "/availability_zones/list", "availability_zone", func(ctx context.Context, op Operations) (any, error) {
			return op.ListAllAvailabilityZones(ctx, "name==keep")
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			for _, total := range []int64{0, 1, 100, 101, 200, 201} {
				t.Run(strconv.FormatInt(total, 10), func(t *testing.T) {
					mux, client, server := setup(t)
					defer server.Close()

					var offsets []int64

					mux.HandleFunc("/api/nutanix/v3"+operation.path, func(writer http.ResponseWriter, r *http.Request) {
						var request DSMetadata
						if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
							http.Error(writer, "invalid fixture request", http.StatusBadRequest)

							return
						}

						assert.Equal(t, "name==keep", ptr.Deref(request.Filter, ""))
						assert.Equal(t, operation.kind, ptr.Deref(request.Kind, ""))
						assert.Equal(t, int64(100), ptr.Deref(request.Length, 0))
						offset := ptr.Deref(request.Offset, 0)
						offsets = append(offsets, offset)

						count := min(int64(100), total-offset)
						if !assert.GreaterOrEqual(t, count, int64(0)) {
							http.Error(writer, "invalid fixture offset", http.StatusBadRequest)

							return
						}

						entities := make([]map[string]any, 0, count)
						for i := range count {
							id := fmt.Sprintf("item-%d", offset+i)
							entities = append(entities, map[string]any{"metadata": map[string]any{"uuid": id}, "value": id, "uuid": id, "address_group": map[string]any{"name": id}})
						}

						assert.NoError(t, json.NewEncoder(writer).Encode(map[string]any{"metadata": map[string]any{"offset": offset, "length": count, "total_matches": total}, "entities": entities}))
					})
					response, err := operation.list(context.Background(), Operations{client: client})
					require.NoError(t, err)
					data, err := json.Marshal(response)
					require.NoError(t, err)

					var result struct {
						Entities []json.RawMessage `json:"entities"`
					}
					require.NoError(t, json.Unmarshal(data, &result))
					require.Len(t, result.Entities, int(total))

					seen := make(map[string]bool)
					for _, entity := range result.Entities {
						require.False(t, seen[string(entity)], "duplicate: %s", entity)
						seen[string(entity)] = true
					}

					expected := []int64{0}
					for offset := int64(100); offset < total; offset += 100 {
						expected = append(expected, offset)
					}

					require.Equal(t, expected, offsets)
				})
			}
		})
	}
}

func TestListAllSubnetFilteredPages(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	var offsets []int64

	mux.HandleFunc("/api/nutanix/v3/subnets/list", func(writer http.ResponseWriter, r *http.Request) {
		var request DSMetadata
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
			http.Error(writer, "invalid fixture request", http.StatusBadRequest)

			return
		}

		offset := ptr.Deref(request.Offset, 0)
		offsets = append(offsets, offset)
		entities := make([]map[string]any, 0)

		for i := offset; i < min(offset+100, int64(201)); i++ {
			name := "drop"
			if i == 200 {
				name = "keep"
			}

			entities = append(entities, map[string]any{"metadata": map[string]any{"uuid": strconv.FormatInt(i, 10)}, "status": map[string]any{"name": name}})
		}

		assert.NoError(t, json.NewEncoder(writer).Encode(map[string]any{"metadata": map[string]any{"offset": offset, "length": len(entities), "total_matches": 201}, "entities": entities}))
	})

	response, err := (Operations{client: client}).ListAllSubnet(context.Background(), "", []*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}})
	require.NoError(t, err)
	require.Len(t, response.Entities, 1)
	require.Equal(t, "200", ptr.Deref(response.Entities[0].Metadata.UUID, ""))
	require.Equal(t, []int64{0, 100, 200}, offsets)
}

func TestListAllInvalidPagination(t *testing.T) {
	for _, test := range []struct {
		name, first, second string
		calls, countDelta   int
	}{
		{"nil_metadata", `null`, `null`, 1, 0},
		{"missing_total", `{"offset":0,"length":0}`, `null`, 1, 0},
		{"missing_length", `{"offset":0,"total_matches":101}`, `null`, 1, 0},
		{"missing_offset", `{"length":100,"total_matches":101}`, `null`, 1, 0},
		{"negative_total", `{"offset":0,"length":0,"total_matches":-1}`, `null`, 1, 0},
		{"negative_length", `{"offset":0,"length":-1,"total_matches":101}`, `null`, 1, 0},
		{"wrong_offset", `{"offset":1,"length":100,"total_matches":101}`, `null`, 1, 0},
		{"oversized_length", `{"offset":0,"length":101,"total_matches":101}`, `null`, 1, 0},
		{"length_beyond_total", `{"offset":0,"length":2,"total_matches":1}`, `null`, 1, 0},
		{"nonprogress", `{"offset":0,"length":0,"total_matches":101}`, `null`, 1, 0},
		{"repeated_page", `{"offset":0,"length":100,"total_matches":101}`, `{"offset":0,"length":1,"total_matches":101}`, 2, 0},
		{"changed_total", `{"offset":0,"length":100,"total_matches":101}`, `{"offset":100,"length":1,"total_matches":102}`, 2, 0},
		{"missing_entities", `{"offset":0,"length":1,"total_matches":1}`, `null`, 1, -1},
		{"excess_entities", `{"offset":0,"length":1,"total_matches":1}`, `null`, 1, 1},
		{"filtered_excess_entities", `{"offset":0,"length":1,"total_matches":1}`, `null`, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			mux, client, server := setup(t)
			defer server.Close()

			calls := 0

			path := "/api/nutanix/v3/vms/list"
			if test.name == "filtered_excess_entities" {
				path = "/api/nutanix/v3/subnets/list"
			}

			mux.HandleFunc(path, func(writer http.ResponseWriter, _ *http.Request) {
				calls++

				metadata := test.first
				if calls > 1 {
					metadata = test.second
				}

				var page ListMetadataOutput
				assert.NoError(t, json.Unmarshal([]byte(metadata), &page))
				count := max(int(ptr.Deref(page.Length, 0))+test.countDelta, 0)

				entities := make([]map[string]any, count)
				for i := range entities {
					entities[i] = map[string]any{"metadata": map[string]any{"uuid": strconv.Itoa(i)}, "status": map[string]any{"name": "keep"}}
				}

				rawEntities, err := json.Marshal(entities)
				assert.NoError(t, err)

				testFprintf(t, writer, `{"metadata":%s,"entities":%s}`, metadata, rawEntities)
			})
			require.NotPanics(t, func() {
				var (
					response any
					err      error
				)
				if test.name == "filtered_excess_entities" {
					response, err = (Operations{client: client}).ListAllSubnet(context.Background(), "", []*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}})
				} else {
					response, err = (Operations{client: client}).ListAllVM(context.Background(), "")
				}

				require.Error(t, err)
				require.Nil(t, response)
			})
			require.Equal(t, test.calls, calls)
		})
	}
}

func TestListAllCancellationAndHTTPError(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	calls := 0

	mux.HandleFunc("/api/nutanix/v3/vms/list", func(writer http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			entities := make([]map[string]any, 100)
			for i := range entities {
				entities[i] = map[string]any{"metadata": map[string]any{"uuid": strconv.Itoa(i)}}
			}

			assert.NoError(t, json.NewEncoder(writer).Encode(map[string]any{"metadata": map[string]any{"offset": 0, "length": 100, "total_matches": 101}, "entities": entities}))

			return
		}

		writer.WriteHeader(http.StatusServiceUnavailable)
	})

	operations := Operations{client: client}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := operations.ListAllVM(ctx, "")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls)

	_, err = operations.ListAllVM(context.Background(), "")

	var responseError *prismgoclient.ResponseError
	require.True(t, errors.As(err, &responseError))
	require.Equal(t, http.StatusServiceUnavailable, responseError.StatusCode)
	require.Equal(t, 2, calls)
}

func TestListAllHostSingleRequest(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	calls := 0

	mux.HandleFunc("/api/nutanix/v3/hosts/list", func(writer http.ResponseWriter, r *http.Request) {
		calls++

		var request DSMetadata
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&request)) {
			http.Error(writer, "invalid fixture request", http.StatusBadRequest)

			return
		}

		assert.Nil(t, request.Length)
		assert.Nil(t, request.Offset)

		entities := make([]map[string]any, 201)
		for i := range entities {
			entities[i] = map[string]any{"metadata": map[string]any{"uuid": strconv.Itoa(i)}}
		}

		assert.NoError(t, json.NewEncoder(writer).Encode(map[string]any{"entities": entities}))
	})

	response, err := (Operations{client: client}).ListAllHost(context.Background())
	require.NoError(t, err)
	require.Len(t, response.Entities, 201)
	require.Equal(t, 1, calls)
}

func TestListAllRecoveryPlanEmptyJSON(t *testing.T) {
	for _, entities := range []string{"[]", "null"} {
		t.Run(entities, func(t *testing.T) {
			mux, client, server := setup(t)
			defer server.Close()

			mux.HandleFunc("/api/nutanix/v3/recovery_plans/list", func(w http.ResponseWriter, _ *http.Request) {
				testFprintf(t, w, `{"metadata":{"offset":0,"length":0,"total_matches":0},"entities":%s}`, entities)
			})

			response, err := (Operations{client: client}).ListAllRecoveryPlans(context.Background(), "")
			require.NoError(t, err)
			data, err := json.Marshal(response)
			require.NoError(t, err)

			var result map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &result))
			require.JSONEq(t, entities, string(result["entities"]))
		})
	}
}

func TestListAllPageRequestPreservation(t *testing.T) {
	filters := []*prismgoclient.AdditionalFilter{{Name: "name", Values: []string{"keep"}}}
	request := DSMetadata{Filter: new("name==keep"), Kind: new("subnet"), SortAttribute: new("name"), SortOrder: new("ASCENDING"), ClientSideFilters: filters}
	calls := 0
	err := listAllPages(context.Background(), request, func(page *DSMetadata) (*ListMetadataOutput, int, error) {
		require.Equal(t, request.Filter, page.Filter)
		require.Equal(t, request.Kind, page.Kind)
		require.Equal(t, request.SortAttribute, page.SortAttribute)
		require.Equal(t, request.SortOrder, page.SortOrder)
		require.Equal(t, filters, page.ClientSideFilters)
		require.Equal(t, int64(100), *page.Length)
		require.Equal(t, int64(calls*50), *page.Offset)
		calls++

		return &ListMetadataOutput{Offset: page.Offset, Length: new(int64(50)), TotalMatches: new(int64(150))}, 50, nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
	require.Nil(t, request.Offset)
	require.Nil(t, request.Length)
}

func TestListAllCancellationBetweenPages(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0

	client.OnRequestCompleted(func(*http.Request, *http.Response, any) { cancel() })
	mux.HandleFunc("/api/nutanix/v3/vms/list", func(writer http.ResponseWriter, _ *http.Request) {
		calls++

		entities := make([]map[string]any, 100)
		for i := range entities {
			entities[i] = map[string]any{"metadata": map[string]any{"uuid": strconv.Itoa(i)}}
		}

		assert.NoError(t, json.NewEncoder(writer).Encode(map[string]any{"metadata": map[string]any{"offset": 0, "length": 100, "total_matches": 101}, "entities": entities}))
	})

	response, err := (Operations{client: client}).ListAllVM(ctx, "")
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, response)
	require.Equal(t, 1, calls)
}
