package square

import (
	"context"
	"fmt"

	squaresdk "github.com/square/square-go-sdk/v3"
)

// GetWholesaleCatalog returns all active items in the configured Wholesale category.
// Item variations with VARIABLE_PRICING are excluded — only fixed-price
// variations are supported for ordering.
func (c *Client) GetWholesaleCatalog(ctx context.Context) ([]*squaresdk.CatalogObject, error) {
	if c.WholesaleCategoryID == "" {
		return nil, fmt.Errorf("square: wholesale category ID is required")
	}

	resp, err := c.SDK.Catalog.SearchItems(ctx, &squaresdk.SearchCatalogItemsRequest{
		CategoryIDs: []string{c.WholesaleCategoryID},
	})
	if err != nil {
		return nil, fmt.Errorf("square: GetWholesaleCatalog: %w", err)
	}

	var items []*squaresdk.CatalogObject
	for _, obj := range resp.Items {
		if obj.Item == nil || obj.Item.ItemData == nil {
			continue
		}
		var fixed []*squaresdk.CatalogObject
		for _, v := range obj.Item.ItemData.Variations {
			if v.ItemVariation == nil || v.ItemVariation.ItemVariationData == nil {
				continue
			}
			pt := v.ItemVariation.ItemVariationData.PricingType
			if pt == nil || *pt != squaresdk.CatalogPricingTypeVariablePricing {
				fixed = append(fixed, v)
			}
		}
		if len(fixed) > 0 {
			obj.Item.ItemData.Variations = fixed
			items = append(items, obj)
		}
	}

	return items, nil
}
