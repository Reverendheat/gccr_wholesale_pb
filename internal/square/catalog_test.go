package square

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

const (
	testCategoryID           = "configured-category-id"
	testGroceryGroupID       = "grocery-group-id"
	testCafeGroupID          = "cafe-group-id"
	testGroceryAttributeID   = "grocery-attribute-id"
	testCafeAttributeID      = "cafe-attribute-id"
	testAllowlistAttributeID = "allowlist-attribute-id"
	testCustomerID           = "customer-1"
)

func newCatalogTestClient(baseURL string) *Client {
	return &Client{
		SDK:                                   squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(baseURL)),
		WholesaleCategoryID:                   testCategoryID,
		WholesaleGroceryGroupID:               testGroceryGroupID,
		WholesaleCafeRestaurantGroupID:        testCafeGroupID,
		WholesaleGroceryAttributeID:           testGroceryAttributeID,
		WholesaleCafeRestaurantAttributeID:    testCafeAttributeID,
		WholesaleCustomerAllowlistAttributeID: testAllowlistAttributeID,
	}
}

func testBooleanAttribute(definitionID string, value bool) map[string]any {
	return map[string]any{
		"custom_attribute_definition_id": definitionID,
		"type":                           "BOOLEAN",
		"boolean_value":                  value,
	}
}

func testAttributes(grocery, cafe bool, allowlist *string) map[string]any {
	attributes := map[string]any{
		"other-app:grocery": testBooleanAttribute(testGroceryAttributeID, grocery),
		"other-app:cafe":    testBooleanAttribute(testCafeAttributeID, cafe),
	}
	if allowlist != nil {
		attributes["other-app:allowlist"] = map[string]any{
			"custom_attribute_definition_id": testAllowlistAttributeID,
			"type":                           "STRING",
			"string_value":                   *allowlist,
		}
	}
	return attributes
}

func TestGetWholesaleCatalog_AccessPolicy(t *testing.T) {
	emptyAllowlist := "[]"
	matchingAllowlist := `["customer-1"]`
	nonmatchingAllowlist := `["customer-2"]`
	malformedAllowlist := `not-json`
	nonArrayAllowlist := `{"customer":"customer-1"}`
	emptyElementAllowlist := `[""]`

	tests := []struct {
		name           string
		groupIDs       []string
		attributes     map[string]any
		wantVisible    bool
		wantAudiences  []WholesaleAudience
		wantInvalidLog bool
	}{
		{
			name: "Grocery-only", groupIDs: []string{testGroceryGroupID},
			attributes: testAttributes(true, false, nil), wantVisible: true,
			wantAudiences: []WholesaleAudience{WholesaleAudienceGrocery},
		},
		{
			name: "Cafe-only", groupIDs: []string{testCafeGroupID},
			attributes: testAttributes(false, true, nil), wantVisible: true,
			wantAudiences: []WholesaleAudience{WholesaleAudienceCafeRestaurant},
		},
		{
			name: "both audiences", groupIDs: []string{testCafeGroupID},
			attributes: testAttributes(true, true, nil), wantVisible: true,
			wantAudiences: []WholesaleAudience{WholesaleAudienceGrocery, WholesaleAudienceCafeRestaurant},
		},
		{
			name: "exclusive allowlist match", groupIDs: nil,
			attributes: testAttributes(true, false, &matchingAllowlist), wantVisible: true,
			wantAudiences: []WholesaleAudience{WholesaleAudienceGrocery},
		},
		{
			name: "exclusive allowlist miss", groupIDs: []string{testGroceryGroupID},
			attributes: testAttributes(true, false, &nonmatchingAllowlist),
		},
		{
			name: "empty allowlist fallback", groupIDs: []string{testGroceryGroupID},
			attributes: testAttributes(true, false, &emptyAllowlist), wantVisible: true,
			wantAudiences: []WholesaleAudience{WholesaleAudienceGrocery},
		},
		{
			name: "no audience", groupIDs: []string{testGroceryGroupID, testCafeGroupID},
			attributes: testAttributes(false, false, nil),
		},
		{
			name: "malformed JSON", groupIDs: []string{testGroceryGroupID},
			attributes: testAttributes(true, false, &malformedAllowlist), wantInvalidLog: true,
		},
		{
			name: "non-array JSON", groupIDs: []string{testGroceryGroupID},
			attributes: testAttributes(true, false, &nonArrayAllowlist), wantInvalidLog: true,
		},
		{
			name: "empty allowlist element", groupIDs: []string{testGroceryGroupID},
			attributes: testAttributes(true, false, &emptyElementAllowlist), wantInvalidLog: true,
		},
		{
			name: "customer with no groups", groupIDs: nil,
			attributes: testAttributes(true, false, nil),
		},
	}

	originalLogOutput := log.Writer()
	defer log.SetOutput(originalLogOutput)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			log.SetOutput(&logs)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v2/customers/" + testCustomerID:
					json.NewEncoder(w).Encode(map[string]any{
						"customer": map[string]any{"id": testCustomerID, "group_ids": tt.groupIDs},
					})
				case "/v2/catalog/search-catalog-items":
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatalf("decode catalog request: %v", err)
					}
					if got := body["category_ids"]; !reflect.DeepEqual(got, []any{testCategoryID}) {
						t.Errorf("category_ids = %#v", got)
					}
					json.NewEncoder(w).Encode(map[string]any{
						"items": []map[string]any{{
							"id": "ITEM1", "type": "ITEM",
							"custom_attribute_values": tt.attributes,
							"item_data": map[string]any{
								"name": "Item 1",
								"variations": []map[string]any{
									{
										"id": "FIXED", "type": "ITEM_VARIATION",
										"item_variation_data": map[string]any{
											"item_id": "ITEM1", "name": "Fixed", "pricing_type": "FIXED_PRICING",
											"price_money": map[string]any{"amount": 500, "currency": "USD"},
										},
									},
									{
										"id": "VARIABLE", "type": "ITEM_VARIATION",
										"item_variation_data": map[string]any{
											"item_id": "ITEM1", "name": "Variable", "pricing_type": "VARIABLE_PRICING",
										},
									},
								},
							},
						}},
					})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			items, err := newCatalogTestClient(srv.URL).GetWholesaleCatalog(context.Background(), testCustomerID)
			if err != nil {
				t.Fatalf("GetWholesaleCatalog: %v", err)
			}
			if tt.wantVisible {
				if len(items) != 1 {
					t.Fatalf("visible items = %d, want 1", len(items))
				}
				if items[0].ID != "ITEM1" || items[0].Type != "ITEM" {
					t.Errorf("item identity = %#v", items[0])
				}
				if !reflect.DeepEqual(items[0].WholesaleAudiences, tt.wantAudiences) {
					t.Errorf("audiences = %#v, want %#v", items[0].WholesaleAudiences, tt.wantAudiences)
				}
				if got := items[0].ItemData.Variations; len(got) != 1 || got[0].ItemVariation.ID != "FIXED" {
					t.Errorf("fixed-price variations = %#v", got)
				}
			} else if len(items) != 0 {
				t.Errorf("visible items = %d, want 0", len(items))
			}
			loggedInvalid := strings.Contains(logs.String(), "square: wholesale item ITEM1 has invalid customer allowlist")
			if loggedInvalid != tt.wantInvalidLog {
				t.Errorf("invalid allowlist log = %v, want %v; logs: %q", loggedInvalid, tt.wantInvalidLog, logs.String())
			}
		})
	}
}

func TestGetWholesaleCatalog_RequiresAllConfiguration(t *testing.T) {
	tests := []struct {
		envName string
		clear   func(*Client)
	}{
		{"SQUARE_WHOLESALE_CATEGORY_ID", func(c *Client) { c.WholesaleCategoryID = "" }},
		{"SQUARE_WHOLESALE_GROCERY_GROUP_ID", func(c *Client) { c.WholesaleGroceryGroupID = "" }},
		{"SQUARE_WHOLESALE_CAFE_RESTAURANT_GROUP_ID", func(c *Client) { c.WholesaleCafeRestaurantGroupID = "" }},
		{"SQUARE_WHOLESALE_GROCERY_ATTRIBUTE_ID", func(c *Client) { c.WholesaleGroceryAttributeID = "" }},
		{"SQUARE_WHOLESALE_CAFE_RESTAURANT_ATTRIBUTE_ID", func(c *Client) { c.WholesaleCafeRestaurantAttributeID = "" }},
		{"SQUARE_WHOLESALE_CUSTOMER_ALLOWLIST_ATTRIBUTE_ID", func(c *Client) { c.WholesaleCustomerAllowlistAttributeID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.envName, func(t *testing.T) {
			client := newCatalogTestClient("http://unused.invalid")
			tt.clear(client)
			items, err := client.GetWholesaleCatalog(context.Background(), testCustomerID)
			if err == nil || !strings.Contains(err.Error(), tt.envName) {
				t.Fatalf("error = %v, want missing %s", err, tt.envName)
			}
			if items != nil {
				t.Fatalf("items = %#v, want nil", items)
			}
		})
	}
}

func TestGetWholesaleCatalog_SquareFailuresReturnNoData(t *testing.T) {
	tests := []struct {
		name         string
		customerCode int
		catalogCode  int
	}{
		{name: "customer failure", customerCode: http.StatusInternalServerError, catalogCode: http.StatusOK},
		{name: "catalog failure", customerCode: http.StatusOK, catalogCode: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v2/customers/" + testCustomerID:
					w.WriteHeader(tt.customerCode)
					if tt.customerCode == http.StatusOK {
						json.NewEncoder(w).Encode(map[string]any{"customer": map[string]any{"id": testCustomerID}})
					}
				case "/v2/catalog/search-catalog-items":
					w.WriteHeader(tt.catalogCode)
					if tt.catalogCode == http.StatusOK {
						json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			items, err := newCatalogTestClient(srv.URL).GetWholesaleCatalog(context.Background(), testCustomerID)
			if err == nil {
				t.Fatal("expected Square error")
			}
			if items != nil {
				t.Fatalf("items = %#v, want nil", items)
			}
		})
	}
}
