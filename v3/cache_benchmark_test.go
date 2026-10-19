package v3

import (
	"net/url"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
)

func BenchmarkClientCacheHitParallel(b *testing.B) {
	cache := NewClientCache()
	params := &cachedClientParams{name: "fixture", mgmtEndpoint: types.ManagementEndpoint{
		Address:        &url.URL{Host: "fixture.invalid"},
		ApiCredentials: types.ApiCredentials{APIKey: "offline-fixture"},
	}}

	want, err := cache.GetOrCreate(params)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			got, err := cache.GetOrCreate(params)
			if err != nil || got != want {
				b.Errorf("cache hit changed client: %v", err)

				return
			}
		}
	})
}
