package v3

import (
	"context"
	"errors"

	"github.com/nutanix-cloud-native/prism-go-client/v3/models"
)

// listAllPages advances by raw response length, including entities removed by client-side filters.
func listAllPages(ctx context.Context, request DSMetadata, fetch func(*DSMetadata) (*ListMetadataOutput, int, error)) error {
	var offset, total int64

	request.Length = new(itemsPerPage)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		request.Offset = new(offset)

		metadata, count, err := fetch(&request)
		if err != nil {
			return err
		}

		if err := validateListMetadata(metadata, offset); err != nil {
			return err
		}

		matches, length := *metadata.TotalMatches, *metadata.Length

		observed := int64(count)
		if observed < 0 || observed > length || (len(request.ClientSideFilters) == 0 && observed != length) {
			return errors.New("list pagination entity count does not match metadata")
		}

		if offset == 0 {
			total = matches
		} else if matches != total {
			return errors.New("list pagination total changed")
		}

		offset += length
		if offset == total {
			return nil
		}

		if length == 0 {
			return errors.New("list pagination made no progress")
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

func validateListMetadata(metadata *ListMetadataOutput, offset int64) error {
	if metadata == nil || metadata.TotalMatches == nil || metadata.Length == nil || metadata.Offset == nil {
		return errors.New("incomplete list pagination metadata")
	}

	matches, length := *metadata.TotalMatches, *metadata.Length
	if matches < 0 || *metadata.Offset != offset || length < 0 || length > itemsPerPage || offset > matches || length > matches-offset {
		return errors.New("invalid list pagination metadata")
	}

	return nil
}
