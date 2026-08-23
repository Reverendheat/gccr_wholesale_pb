package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/reverendheat/gccr_invoice/internal/square"
	squareclient "github.com/square/square-go-sdk/v3/client"
	"github.com/square/square-go-sdk/v3/option"
)

func TestStaffCanReadAndUpdateCustomerAudiences(t *testing.T) {
	app, self, target, unlinked, staff := newCatalogAccessTestApp(t)
	memberships := map[string]bool{"GROCERY_GROUP": true, "CAFE_GROUP": false, "UNRELATED": true}
	var groupWrites []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v2/customers/SQ_TARGET":
			groupIDs := make([]string, 0, len(memberships))
			for groupID, enabled := range memberships {
				if enabled {
					groupIDs = append(groupIDs, groupID)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"customer": map[string]any{
				"id": "SQ_TARGET", "group_ids": groupIDs,
			}})
		case strings.HasPrefix(r.URL.Path, "/v2/customers/SQ_TARGET/groups/"):
			groupID := strings.TrimPrefix(r.URL.Path, "/v2/customers/SQ_TARGET/groups/")
			groupWrites = append(groupWrites, r.Method+" "+groupID)
			memberships[groupID] = r.Method == http.MethodPut
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	sq := &square.Client{
		SDK:                            squareclient.NewClient(option.WithToken("tok"), option.WithBaseURL(srv.URL)),
		WholesaleGroceryGroupID:        "GROCERY_GROUP",
		WholesaleCafeRestaurantGroupID: "CAFE_GROUP",
	}

	recorder, err := invokeCatalogAccessHandler(
		t, app, staff, http.MethodGet, "/customers/"+target.Id+"/audiences", nil,
		map[string]string{"id": target.Id}, handleGetCustomerAudiences(sq),
	)
	if err != nil {
		t.Fatal(err)
	}
	var access customerAudienceAccess
	if err := json.NewDecoder(recorder.Body).Decode(&access); err != nil {
		t.Fatal(err)
	}
	if !access.Grocery || access.CafeRestaurant {
		t.Fatalf("initial access = %+v", access)
	}

	recorder, err = invokeCatalogAccessHandler(
		t, app, staff, http.MethodPatch, "/customers/"+target.Id+"/audiences",
		map[string]any{"grocery": false, "cafe_restaurant": true},
		map[string]string{"id": target.Id}, handleUpdateCustomerAudiences(sq),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(recorder.Body).Decode(&access); err != nil {
		t.Fatal(err)
	}
	if access.Grocery || !access.CafeRestaurant {
		t.Fatalf("updated access = %+v", access)
	}
	if len(groupWrites) != 2 || groupWrites[0] != "DELETE GROCERY_GROUP" || groupWrites[1] != "PUT CAFE_GROUP" {
		t.Fatalf("group writes = %v", groupWrites)
	}
	if !memberships["UNRELATED"] {
		t.Fatal("unrelated Square group was modified")
	}

	_, err = invokeCatalogAccessHandler(
		t, app, self, http.MethodGet, "/customers/"+target.Id+"/audiences", nil,
		map[string]string{"id": target.Id}, handleGetCustomerAudiences(sq),
	)
	if apiErr := router.ToApiError(err); apiErr.Status != http.StatusForbidden {
		t.Fatalf("customer authorization error = %#v", apiErr)
	}

	_, err = invokeCatalogAccessHandler(
		t, app, staff, http.MethodGet, "/customers/"+unlinked.Id+"/audiences", nil,
		map[string]string{"id": unlinked.Id}, handleGetCustomerAudiences(sq),
	)
	apiErr := router.ToApiError(err)
	if apiErr.Status != http.StatusBadRequest || apiErr.Message != "Customer is not linked to Square." {
		t.Fatalf("unlinked customer error = %#v", apiErr)
	}
}
