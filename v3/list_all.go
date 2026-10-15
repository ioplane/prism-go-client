package v3

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/prism-go-client/v3/models"
)

// listAllPages advances by raw response length, including entities removed by client-side filters.
func listAllPages(ctx context.Context, request DSMetadata, fetch func(*DSMetadata) (*ListMetadataOutput, error)) error {
	var offset, total int64
	request.Length = ptr.To(itemsPerPage)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		request.Offset = ptr.To(offset)
		metadata, err := fetch(&request)
		if err != nil {
			return err
		}
		if metadata == nil || metadata.TotalMatches == nil || metadata.Length == nil || metadata.Offset == nil {
			return fmt.Errorf("incomplete list pagination metadata")
		}
		matches, length := *metadata.TotalMatches, *metadata.Length
		if matches < 0 || *metadata.Offset != offset || length < 0 || length > itemsPerPage || offset > matches || length > matches-offset {
			return fmt.Errorf("invalid list pagination metadata")
		}
		if offset == 0 {
			total = matches
		} else if matches != total {
			return fmt.Errorf("list pagination total changed")
		}
		offset += length
		if offset == total {
			return nil
		}
		if length == 0 {
			return fmt.Errorf("list pagination made no progress")
		}
	}
}

func recoveryPlanPageMetadata(metadata *models.RecoveryPlanListMetadataOutput) *ListMetadataOutput {
	if metadata == nil {
		return nil
	}
	// Generated scalar fields cannot distinguish omitted values from zero.
	return &ListMetadataOutput{Offset: &metadata.Offset, Length: &metadata.Length, TotalMatches: &metadata.TotalMatches}
}
