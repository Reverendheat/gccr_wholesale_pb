package main

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	_ "github.com/reverendheat/gccr_invoice/pb_migrations"

	"github.com/reverendheat/gccr_invoice/internal/config"
	"github.com/reverendheat/gccr_invoice/internal/delivery"
	"github.com/reverendheat/gccr_invoice/internal/hooks"
	"github.com/reverendheat/gccr_invoice/internal/routes"
	"github.com/reverendheat/gccr_invoice/internal/scheduler"
	"github.com/reverendheat/gccr_invoice/internal/square"
)

func main() {
	app := pocketbase.New()

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		return config.ApplySettingsFromEnv(app)
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	sqCfg := square.Config{
		AccessToken:                           os.Getenv("SQUARE_ACCESS_TOKEN"),
		Sandbox:                               os.Getenv("SQUARE_SANDBOX") == "true",
		WholesaleCategoryID:                   os.Getenv("SQUARE_WHOLESALE_CATEGORY_ID"),
		WholesaleGroceryGroupID:               os.Getenv("SQUARE_WHOLESALE_GROCERY_GROUP_ID"),
		WholesaleCafeRestaurantGroupID:        os.Getenv("SQUARE_WHOLESALE_CAFE_RESTAURANT_GROUP_ID"),
		WholesaleGroceryAttributeID:           os.Getenv("SQUARE_WHOLESALE_GROCERY_ATTRIBUTE_ID"),
		WholesaleCafeRestaurantAttributeID:    os.Getenv("SQUARE_WHOLESALE_CAFE_RESTAURANT_ATTRIBUTE_ID"),
		WholesaleCustomerAllowlistAttributeID: os.Getenv("SQUARE_WHOLESALE_CUSTOMER_ALLOWLIST_ATTRIBUTE_ID"),
		WholesaleGrindModifierListID:          os.Getenv("SQUARE_WHOLESALE_GRIND_MODIFIER_LIST_ID"),
		WholesaleDripModifierID:               os.Getenv("SQUARE_WHOLESALE_DRIP_MODIFIER_ID"),
	}
	if sqCfg.AccessToken == "" {
		log.Fatal("SQUARE_ACCESS_TOKEN environment variable is required")
	}
	if sqCfg.WholesaleCategoryID == "" {
		log.Fatal("SQUARE_WHOLESALE_CATEGORY_ID environment variable is required")
	}
	if sqCfg.WholesaleGroceryGroupID == "" {
		log.Fatal("SQUARE_WHOLESALE_GROCERY_GROUP_ID environment variable is required")
	}
	if sqCfg.WholesaleCafeRestaurantGroupID == "" {
		log.Fatal("SQUARE_WHOLESALE_CAFE_RESTAURANT_GROUP_ID environment variable is required")
	}
	if sqCfg.WholesaleGroceryAttributeID == "" {
		log.Fatal("SQUARE_WHOLESALE_GROCERY_ATTRIBUTE_ID environment variable is required")
	}
	if sqCfg.WholesaleCafeRestaurantAttributeID == "" {
		log.Fatal("SQUARE_WHOLESALE_CAFE_RESTAURANT_ATTRIBUTE_ID environment variable is required")
	}
	if sqCfg.WholesaleCustomerAllowlistAttributeID == "" {
		log.Fatal("SQUARE_WHOLESALE_CUSTOMER_ALLOWLIST_ATTRIBUTE_ID environment variable is required")
	}
	if sqCfg.WholesaleGrindModifierListID == "" {
		log.Fatal("SQUARE_WHOLESALE_GRIND_MODIFIER_LIST_ID environment variable is required")
	}
	if sqCfg.WholesaleDripModifierID == "" {
		log.Fatal("SQUARE_WHOLESALE_DRIP_MODIFIER_ID environment variable is required")
	}

	locationID := os.Getenv("SQUARE_LOCATION_ID")
	if locationID == "" {
		log.Fatal("SQUARE_LOCATION_ID environment variable is required")
	}

	orsAPIKey := os.Getenv("ORS_API_KEY")
	if orsAPIKey == "" {
		log.Fatal("ORS_API_KEY environment variable is required")
	}
	deliveryPolicy, err := delivery.PolicyFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	sq := square.New(sqCfg)
	deliveryService := delivery.NewService(
		locationID,
		sq,
		delivery.NewOpenRouteService(orsAPIKey, "", nil),
		deliveryPolicy,
	)
	hooks.Register(app, sq)
	scheduler.Register(app, sq, deliveryService)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		routes.Register(se, sq, locationID, deliveryService)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), true))
		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
