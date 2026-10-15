package v3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	"net/http"
	"testing"
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
				t.Run(fmt.Sprint(total), func(t *testing.T) {
					mux, client, server := setup(t)
					defer server.Close()
					var offsets []int64
					mux.HandleFunc("/api/nutanix/v3"+operation.path, func(w http.ResponseWriter, r *http.Request) {
						var request DSMetadata
						require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
						require.Equal(t, "name==keep", ptr.Deref(request.Filter, ""))
						require.Equal(t, operation.kind, ptr.Deref(request.Kind, ""))
						require.Equal(t, int64(100), ptr.Deref(request.Length, 0))
						offset := ptr.Deref(request.Offset, 0)
						offsets = append(offsets, offset)
						count := min(int64(100), total-offset)
						require.GreaterOrEqual(t, count, int64(0))
						entities := make([]map[string]any, 0, count)
						for i := int64(0); i < count; i++ {
							id := fmt.Sprintf("item-%d", offset+i)
							entities = append(entities, map[string]any{"metadata": map[string]any{"uuid": id}, "value": id, "uuid": id, "address_group": map[string]any{"name": id}})
						}
						require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"metadata": map[string]any{"offset": offset, "length": count, "total_matches": total}, "entities": entities}))
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
	mux.HandleFunc("/api/nutanix/v3/subnets/list", func(w http.ResponseWriter, r *http.Request) {
		var request DSMetadata
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		offset := ptr.Deref(request.Offset, 0)
		offsets = append(offsets, offset)
		entities := make([]map[string]any, 0)
		for i := offset; i < min(offset+100, int64(201)); i++ {
			name := "drop"
			if i == 200 {
				name = "keep"
			}
			entities = append(entities, map[string]any{"metadata": map[string]any{"uuid": fmt.Sprint(i)}, "status": map[string]any{"name": name}})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"metadata": map[string]any{"offset": offset, "length": len(entities), "total_matches": 201}, "entities": entities}))
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
		calls               int
	}{
		{"nil_metadata", `null`, `null`, 1},
		{"missing_total", `{"offset":0,"length":0}`, `null`, 1},
		{"missing_length", `{"offset":0,"total_matches":101}`, `null`, 1},
		{"missing_offset", `{"length":100,"total_matches":101}`, `null`, 1},
		{"negative_total", `{"offset":0,"length":0,"total_matches":-1}`, `null`, 1},
		{"negative_length", `{"offset":0,"length":-1,"total_matches":101}`, `null`, 1},
		{"wrong_offset", `{"offset":1,"length":100,"total_matches":101}`, `null`, 1},
		{"oversized_length", `{"offset":0,"length":101,"total_matches":101}`, `null`, 1},
		{"length_beyond_total", `{"offset":0,"length":2,"total_matches":1}`, `null`, 1},
		{"nonprogress", `{"offset":0,"length":0,"total_matches":101}`, `null`, 1},
		{"repeated_page", `{"offset":0,"length":100,"total_matches":101}`, `{"offset":0,"length":1,"total_matches":101}`, 2},
		{"changed_total", `{"offset":0,"length":100,"total_matches":101}`, `{"offset":100,"length":1,"total_matches":102}`, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			mux, client, server := setup(t)
			defer server.Close()
			calls := 0
			mux.HandleFunc("/api/nutanix/v3/vms/list", func(w http.ResponseWriter, r *http.Request) {
				calls++
				metadata := test.first
				if calls > 1 {
					metadata = test.second
				}
				_, _ = fmt.Fprintf(w, `{"metadata":%s,"entities":[]}`, metadata)
			})
			require.NotPanics(t, func() {
				response, err := (Operations{client: client}).ListAllVM(context.Background(), "")
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
	mux.HandleFunc("/api/nutanix/v3/vms/list", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = fmt.Fprint(w, `{"metadata":{"offset":0,"length":100,"total_matches":101},"entities":[]}`)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	op := Operations{client: client}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := op.ListAllVM(ctx, "")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls)
	_, err = op.ListAllVM(context.Background(), "")
	var responseError *prismgoclient.ResponseError
	require.True(t, errors.As(err, &responseError))
	require.Equal(t, http.StatusServiceUnavailable, responseError.StatusCode)
	require.Equal(t, 2, calls)
}

func TestListAllHostSingleRequest(t *testing.T) {
	mux, client, server := setup(t)
	defer server.Close()
	calls := 0
	mux.HandleFunc("/api/nutanix/v3/hosts/list", func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request DSMetadata
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Nil(t, request.Length)
		require.Nil(t, request.Offset)
		entities := make([]map[string]any, 201)
		for i := range entities {
			entities[i] = map[string]any{"metadata": map[string]any{"uuid": fmt.Sprint(i)}}
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"entities": entities}))
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
			mux.HandleFunc("/api/nutanix/v3/recovery_plans/list", func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"metadata":{"offset":0,"length":0,"total_matches":0},"entities":%s}`, entities)
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
	request := DSMetadata{Filter: ptr.To("name==keep"), Kind: ptr.To("subnet"), SortAttribute: ptr.To("name"), SortOrder: ptr.To("ASCENDING"), ClientSideFilters: filters}
	calls := 0
	err := listAllPages(context.Background(), request, func(page *DSMetadata) (*ListMetadataOutput, error) {
		require.Equal(t, request.Filter, page.Filter)
		require.Equal(t, request.Kind, page.Kind)
		require.Equal(t, request.SortAttribute, page.SortAttribute)
		require.Equal(t, request.SortOrder, page.SortOrder)
		require.Equal(t, filters, page.ClientSideFilters)
		require.Equal(t, int64(100), *page.Length)
		require.Equal(t, int64(calls*50), *page.Offset)
		calls++
		return &ListMetadataOutput{Offset: page.Offset, Length: ptr.To(int64(50)), TotalMatches: ptr.To(int64(150))}, nil
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
	client.OnRequestCompleted(func(*http.Request, *http.Response, interface{}) { cancel() })
	mux.HandleFunc("/api/nutanix/v3/vms/list", func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = fmt.Fprint(w, `{"metadata":{"offset":0,"length":100,"total_matches":101},"entities":[]}`)
	})
	response, err := (Operations{client: client}).ListAllVM(ctx, "")
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, response)
	require.Equal(t, 1, calls)
}
