package square

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	squaresdk "github.com/square/square-go-sdk/v3"
)

type WholesaleAudience string

const (
	WholesaleAudienceGrocery        WholesaleAudience = "grocery"
	WholesaleAudienceCafeRestaurant WholesaleAudience = "cafe_restaurant"
)

type GrindOption struct {
	ModifierID string `json:"modifier_id,omitempty"`
	Name       string `json:"name"`
}

type WholesaleCatalogItem struct {
	ID                 string                 `json:"id"`
	Type               string                 `json:"type"`
	ItemData           *squaresdk.CatalogItem `json:"item_data"`
	WholesaleAudiences []WholesaleAudience    `json:"wholesale_audiences"`
	GrindOptions       []GrindOption          `json:"grind_options,omitempty"`
}

func (c *Client) validateWholesaleCatalogConfig() error {
	required := []struct {
		name  string
		value string
	}{
		{"SQUARE_WHOLESALE_CATEGORY_ID", c.WholesaleCategoryID},
		{"SQUARE_WHOLESALE_GROCERY_GROUP_ID", c.WholesaleGroceryGroupID},
		{"SQUARE_WHOLESALE_CAFE_RESTAURANT_GROUP_ID", c.WholesaleCafeRestaurantGroupID},
		{"SQUARE_WHOLESALE_GROCERY_ATTRIBUTE_ID", c.WholesaleGroceryAttributeID},
		{"SQUARE_WHOLESALE_CAFE_RESTAURANT_ATTRIBUTE_ID", c.WholesaleCafeRestaurantAttributeID},
		{"SQUARE_WHOLESALE_CUSTOMER_ALLOWLIST_ATTRIBUTE_ID", c.WholesaleCustomerAllowlistAttributeID},
	}
	for _, field := range required {
		if field.value == "" {
			return fmt.Errorf("square: %s is required", field.name)
		}
	}
	return nil
}

func wholesaleAudiences(
	values map[string]*squaresdk.CatalogCustomAttributeValue,
	groceryDefinitionID string,
	cafeRestaurantDefinitionID string,
) []WholesaleAudience {
	var grocery, cafeRestaurant bool
	for _, value := range values {
		if value == nil || value.CustomAttributeDefinitionID == nil || value.BooleanValue == nil || !*value.BooleanValue {
			continue
		}
		switch *value.CustomAttributeDefinitionID {
		case groceryDefinitionID:
			grocery = true
		case cafeRestaurantDefinitionID:
			cafeRestaurant = true
		}
	}

	audiences := make([]WholesaleAudience, 0, 2)
	if grocery {
		audiences = append(audiences, WholesaleAudienceGrocery)
	}
	if cafeRestaurant {
		audiences = append(audiences, WholesaleAudienceCafeRestaurant)
	}
	return audiences
}

func customerAllowlist(
	values map[string]*squaresdk.CatalogCustomAttributeValue,
	definitionID string,
	customerID string,
) (exclusive bool, allowed bool, valid bool) {
	for _, value := range values {
		if value == nil || value.CustomAttributeDefinitionID == nil || *value.CustomAttributeDefinitionID != definitionID ||
			value.StringValue == nil || strings.TrimSpace(*value.StringValue) == "" {
			continue
		}

		var customerIDs []string
		if err := json.Unmarshal([]byte(*value.StringValue), &customerIDs); err != nil {
			return false, false, false
		}
		if len(customerIDs) == 0 {
			return false, false, true
		}
		for _, allowedCustomerID := range customerIDs {
			if strings.TrimSpace(allowedCustomerID) == "" {
				return false, false, false
			}
			if allowedCustomerID == customerID {
				allowed = true
			}
		}
		return true, allowed, true
	}
	return false, false, true
}

func (c *Client) addGrindOptions(ctx context.Context, items []WholesaleCatalogItem) error {
	modifierListIDs := map[string]struct{}{}
	for _, item := range items {
		for _, info := range item.ItemData.ModifierListInfo {
			if info != nil && info.ModifierListID != "" {
				modifierListIDs[info.ModifierListID] = struct{}{}
			}
		}
	}
	if len(modifierListIDs) == 0 {
		return nil
	}

	ids := make([]string, 0, len(modifierListIDs))
	for id := range modifierListIDs {
		ids = append(ids, id)
	}
	resp, err := c.SDK.Catalog.BatchGet(ctx, &squaresdk.BatchGetCatalogObjectsRequest{
		ObjectIDs: ids,
	})
	if err != nil {
		return fmt.Errorf("square: load catalog modifiers: %w", err)
	}

	optionsByListID := map[string][]GrindOption{}
	for _, obj := range resp.Objects {
		if obj == nil || obj.ModifierList == nil || obj.ModifierList.ModifierListData == nil {
			continue
		}
		data := obj.ModifierList.ModifierListData
		if data.Name == nil || !strings.EqualFold(strings.TrimSpace(*data.Name), "Ground") {
			continue
		}
		for _, modifier := range data.Modifiers {
			if modifier == nil || modifier.Modifier == nil || modifier.Modifier.ModifierData == nil ||
				modifier.Modifier.ModifierData.Name == nil ||
				!strings.EqualFold(strings.TrimSpace(*modifier.Modifier.ModifierData.Name), "Drip") {
				continue
			}
			optionsByListID[obj.ModifierList.ID] = []GrindOption{
				{Name: "Whole Bean"},
				{ModifierID: modifier.Modifier.ID, Name: "Drip"},
			}
			break
		}
	}

	for i := range items {
		for _, info := range items[i].ItemData.ModifierListInfo {
			if info == nil {
				continue
			}
			if options := optionsByListID[info.ModifierListID]; len(options) > 0 {
				items[i].GrindOptions = append([]GrindOption(nil), options...)
				break
			}
		}
	}
	return nil
}

// GetWholesaleCatalog returns fixed-price items visible to the target Square customer.
func (c *Client) GetWholesaleCatalog(ctx context.Context, squareCustomerID string) ([]WholesaleCatalogItem, error) {
	if err := c.validateWholesaleCatalogConfig(); err != nil {
		return nil, err
	}

	customer, err := c.GetCustomer(ctx, squareCustomerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, fmt.Errorf("square: GetWholesaleCatalog: customer %q was not returned", squareCustomerID)
	}

	resp, err := c.SDK.Catalog.SearchItems(ctx, &squaresdk.SearchCatalogItemsRequest{
		CategoryIDs: []string{c.WholesaleCategoryID},
	})
	if err != nil {
		return nil, fmt.Errorf("square: GetWholesaleCatalog: %w", err)
	}

	groupIDs := make(map[string]struct{}, len(customer.GroupIDs))
	for _, groupID := range customer.GroupIDs {
		groupIDs[groupID] = struct{}{}
	}

	items := make([]WholesaleCatalogItem, 0, len(resp.Items))
	for _, obj := range resp.Items {
		if obj == nil || obj.Item == nil || obj.Item.ItemData == nil {
			continue
		}

		audiences := wholesaleAudiences(
			obj.Item.CustomAttributeValues,
			c.WholesaleGroceryAttributeID,
			c.WholesaleCafeRestaurantAttributeID,
		)
		exclusive, allowed, valid := customerAllowlist(
			obj.Item.CustomAttributeValues,
			c.WholesaleCustomerAllowlistAttributeID,
			squareCustomerID,
		)
		if !valid {
			log.Printf("square: wholesale item %s has invalid customer allowlist", obj.Item.ID)
			continue
		}
		if len(audiences) == 0 {
			continue
		}
		if exclusive {
			if !allowed {
				continue
			}
		} else {
			visible := false
			for _, audience := range audiences {
				switch audience {
				case WholesaleAudienceGrocery:
					_, visible = groupIDs[c.WholesaleGroceryGroupID]
				case WholesaleAudienceCafeRestaurant:
					_, visible = groupIDs[c.WholesaleCafeRestaurantGroupID]
				}
				if visible {
					break
				}
			}
			if !visible {
				continue
			}
		}

		fixed := make([]*squaresdk.CatalogObject, 0, len(obj.Item.ItemData.Variations))
		for _, variation := range obj.Item.ItemData.Variations {
			if variation == nil || variation.ItemVariation == nil || variation.ItemVariation.ItemVariationData == nil {
				continue
			}
			pricingType := variation.ItemVariation.ItemVariationData.PricingType
			if pricingType == nil || *pricingType != squaresdk.CatalogPricingTypeVariablePricing {
				fixed = append(fixed, variation)
			}
		}
		if len(fixed) == 0 {
			continue
		}
		obj.Item.ItemData.Variations = fixed
		items = append(items, WholesaleCatalogItem{
			ID:                 obj.Item.ID,
			Type:               obj.Type,
			ItemData:           obj.Item.ItemData,
			WholesaleAudiences: audiences,
		})
	}
	if err := c.addGrindOptions(ctx, items); err != nil {
		return nil, err
	}

	return items, nil
}
