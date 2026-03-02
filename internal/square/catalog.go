package square

import (
	"context"
	"fmt"
)

const wholesaleCategoryID = "XOCS7UGPDHGI62BVN5FXHR5W"

// Money represents a Square monetary amount.
type Money struct {
	Amount   int64  `json:"amount"`   // in smallest currency unit (cents for USD)
	Currency string `json:"currency"` // e.g. "USD"
}

// CatalogItemVariation is the orderable unit of a catalog item.
type CatalogItemVariation struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	ItemVariationData struct {
		ItemID      string `json:"item_id"`
		Name        string `json:"name"`
		PricingType string `json:"pricing_type"` // FIXED_PRICING or VARIABLE_PRICING
		PriceMoney  *Money `json:"price_money"`
	} `json:"item_variation_data"`
}

// CatalogItem is a single item in the Square catalog.
type CatalogItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	ItemData struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Variations  []CatalogItemVariation `json:"variations"`
	} `json:"item_data"`
}

type searchCatalogItemsRequest struct {
	CategoryIDs []string `json:"category_ids"`
}

type searchCatalogItemsResponse struct {
	Items []CatalogItem `json:"items"`
}

// GetWholesaleCatalog returns all active items in the Wholesale category.
// Item variations with VARIABLE_PRICING are excluded — only fixed-price
// variations are supported for ordering.
func (c *Client) GetWholesaleCatalog(ctx context.Context) ([]CatalogItem, error) {
	body := searchCatalogItemsRequest{
		CategoryIDs: []string{wholesaleCategoryID},
	}

	var resp searchCatalogItemsResponse
	if err := c.doPost(ctx, "/v2/catalog/search-catalog-items", body, &resp); err != nil {
		return nil, fmt.Errorf("square: GetWholesaleCatalog: %w", err)
	}

	// Strip variable-priced variations; drop items that have no orderable variations left.
	var items []CatalogItem
	for _, item := range resp.Items {
		var fixed []CatalogItemVariation
		for _, v := range item.ItemData.Variations {
			if v.ItemVariationData.PricingType != "VARIABLE_PRICING" {
				fixed = append(fixed, v)
			}
		}
		if len(fixed) > 0 {
			item.ItemData.Variations = fixed
			items = append(items, item)
		}
	}

	return items, nil
}
