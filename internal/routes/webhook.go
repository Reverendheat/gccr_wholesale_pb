package routes

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	"github.com/reverendheat/gccr_invoice/internal/orders/fsm"
)

// squareWebhookEvent is the envelope Square sends for all event types.
type squareWebhookEvent struct {
	Type string                 `json:"type"`
	Data squareWebhookEventData `json:"data"`
}

type squareWebhookEventData struct {
	Object squareWebhookObject `json:"object"`
}

type squareWebhookObject struct {
	Invoice squareWebhookInvoice `json:"invoice"`
}

type squareWebhookInvoice struct {
	ID      string `json:"id"`
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// registerWebhook adds the Square webhook endpoint to the router.
// It must be registered outside of any auth-gated group.
func registerWebhook(se *core.ServeEvent, signatureKey, notificationURL string) {
	se.Router.POST("/api/webhooks/square", handleSquareWebhook(signatureKey, notificationURL))
}

func handleSquareWebhook(signatureKey, notificationURL string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		rawBody, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return e.BadRequestError("Could not read request body", err)
		}
		// Restore body so downstream reads still work if needed.
		e.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

		// Verify Square HMAC-SHA256 signature when a key is configured.
		if signatureKey != "" {
			if err := verifySquareSignature(signatureKey, notificationURL, rawBody, e.Request.Header.Get("x-square-hmacsha256-signature")); err != nil {
				log.Printf("square webhook: signature verification failed: %v", err)
				return e.JSON(http.StatusUnauthorized, map[string]any{"error": "invalid signature"})
			}
		} else {
			log.Print("square webhook: SQUARE_WEBHOOK_SIGNATURE_KEY not set, skipping signature verification")
		}

		var event squareWebhookEvent
		if err := json.Unmarshal(rawBody, &event); err != nil {
			return e.BadRequestError("Could not parse webhook payload", err)
		}

		if workflowEvent, ok := squareInvoiceEvent(event.Type); ok {
			if err := handleInvoiceEvent(e, event.Data.Object.Invoice.ID, workflowEvent); err != nil {
				log.Printf("square webhook: %s: %v", event.Type, err)
				return e.InternalServerError("Could not reconcile Square invoice", err)
			}
		}

		return e.JSON(http.StatusOK, map[string]any{"ok": true})
	}
}

func squareInvoiceEvent(eventType string) (fsm.Event, bool) {
	switch eventType {
	case "invoice.payment_made":
		return fsm.EventSquareInvoicePaid, true
	case "invoice.canceled":
		return fsm.EventSquareInvoiceCancelled, true
	case "invoice.refunded":
		return fsm.EventSquareNeedsReview, true
	default:
		return "", false
	}
}

func handleInvoiceEvent(e *core.RequestEvent, squareInvoiceID string, event fsm.Event) error {
	if squareInvoiceID == "" {
		return fmt.Errorf("missing invoice ID in payload")
	}

	records, err := e.App.FindRecordsByFilter(
		"orders",
		fmt.Sprintf("squareInvoiceId = '%s'", squareInvoiceID),
		"", 1, 0,
	)
	if err != nil {
		return fmt.Errorf("find order for squareInvoiceId %s: %w", squareInvoiceID, err)
	}
	if len(records) == 0 {
		return nil // Webhook can include unrelated invoices from the Square account.
	}

	order := records[0]
	next, err := fsm.Apply(fsm.Status(order.GetString("status")), event)
	if err != nil {
		return fmt.Errorf("apply %s event: %w", event, err)
	}
	order.Set("status", string(next))
	if err := e.App.Save(order); err != nil {
		return fmt.Errorf("save order: %w", err)
	}

	log.Printf("square webhook: order %s applied %s (invoice %s)", order.Id, event, squareInvoiceID)
	return nil
}

// verifySquareSignature checks the HMAC-SHA256 signature Square includes on
// every webhook delivery. See:
// https://developer.squareup.com/docs/webhooks/step3validate
func verifySquareSignature(signatureKey, notificationURL string, body []byte, receivedSig string) error {
	mac := hmac.New(sha256.New, []byte(signatureKey))
	mac.Write([]byte(notificationURL))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(receivedSig)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
