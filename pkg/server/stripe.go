package server

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/checkout/session"
	"github.com/stripe/stripe-go/v79/paymentintent"
	"github.com/stripe/stripe-go/v79/webhook"
	"io"
	"math/rand"
	"net/http"
	"pdfinspector/pkg/filesystem"
	"strings"
)

//methods for a stripe integration. probably a lot of copypasta from docs/gpt due to laziness.
//
//func main() {
//	// This is your test secret API key.
//	stripe.Key = "sk_test_51Q2PXFRpCqWuwLGKWCJ4CYDOlW6EWlaawYpVINt8Xaa0HSl49cj1vdEZoLCwixbo1IaI8PbVi95yOADv1hHS4pnD00eal2IIyi"
//
//	fs := http.FileServer(http.Dir("public"))
//	http.Handle("/", fs)
//	http.HandleFunc("/create-payment-intent", handleCreatePaymentIntent)
//
//	addr := "localhost:4242"
//	log.Info().Msgf("Listening on %s ...", addr)
//	log.Fatal(http.ListenAndServe(addr, nil))
//}

type item struct {
	Id     string
	Amount int64
}

func calculateOrderAmount(items []item) int64 {
	// Calculate the order total on the server to prevent
	// people from directly manipulating the amount on the client
	total := int64(0)
	for _, itemEntry := range items {
		total += itemEntry.Amount
	}
	return total
}

func (s *pdfInspectorServer) handleCreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	//if r.Method != "POST" {
	//	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	//	return
	//}

	var req struct {
		Items []item `json:"items"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error().Msgf("json.NewDecoder.Decode: %v", err)
		return
	}

	// Create a PaymentIntent with amount and currency
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(calculateOrderAmount(req.Items)),
		Currency: stripe.String(string(stripe.CurrencyCAD)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	pi, err := paymentintent.New(params)
	log.Trace().Msgf("pi.New: %v", pi.ClientSecret)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error().Msgf("pi.New: %v", err)
		return
	}

	writeJSON(w, struct {
		ClientSecret   string `json:"clientSecret"`
		DpmCheckerLink string `json:"dpmCheckerLink"`
	}{
		ClientSecret: pi.ClientSecret,
		// [DEV]: For demo purposes only, you should avoid exposing the PaymentIntent ID in the client-side code.
		DpmCheckerLink: fmt.Sprintf("https://dashboard.stripe.com/settings/payment_methods/review?transaction_id=%s", pi.ID),
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error().Msgf("json.NewEncoder.Encode: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		log.Error().Msgf("io.Copy: %v", err)
		return
	}
}

func (s *pdfInspectorServer) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(65536)))
	if err != nil {
		log.Trace().Msgf("Error reading request body: %v\n", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if s.config.StripeWebhookSecret == "" {
		log.Trace().Msg("Missing server config for StripeWebhookSecret, will not be able to validate - rejecting request.")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Pass the request body and Stripe-Signature header to ConstructEvent, along
	// with the webhook signing key.
	log.Trace().Msgf("handleStripeWebhook got this payload (pre-verification): %s", string(payload))
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"),
		s.config.StripeWebhookSecret)
	log.Trace().Msgf("handleStripeWebhook got this payload (decoded/verified): %#v", event)

	if err != nil {
		log.Trace().Msgf("Error verifying webhook signature: %v\n", err)
		w.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
		return
	}

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "checkout.session.completed":
		log.Info().Msgf("checkout.session.completed happened with: %s", string(event.Data.Raw))
		var checkoutSession stripe.CheckoutSession
		//var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &checkoutSession)
		if err != nil {
			log.Trace().Msgf("Error parsing webhook JSON: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("checkout.session.completed happened with event ID %s and session ID %s", event.ID, checkoutSession.ID)
		log.Info().Msgf("checkout.session.completed happened with (decoded): %#v", checkoutSession)

		if checkoutSession.PaymentStatus != "paid" {
			//why are we here then? f*** off lol.
			log.Trace().Msgf("Got a webhook for checkout.session.completed without being paid. Buh-bye.")
			w.WriteHeader(http.StatusOK) //
			return
		}

		// Retrieve the line items (products/services) for the checkout session (requires us to have set the stripe.Key API key value during server startup)
		//log.Info().Msgf("checkout.session.completed stripe.Key is: %s", stripe.Key)
		lineItems := session.ListLineItems(&stripe.CheckoutSessionListLineItemsParams{
			Session: stripe.String(checkoutSession.ID),
		})
		totalCreditsToIssue := int64(0)
		for lineItems.Next() {
			// Get the current line item
			lineItem := lineItems.LineItem()
			//log.Info().Msgf("lineitem: %#v", lineItem)

			// Access details for each line item
			//log.Info().Msgf("Product: %s\n", lineItem.Description)
			//log.Info().Msgf("Quantity: %d\n", lineItem.Quantity)
			totalCreditsToIssue += lineItem.Quantity
			//log.Info().Msgf("Price: %d\n", lineItem.Price.UnitAmount)
		}
		//log.Info().Msgf("checkout.session.completed smth: %#v", smth)
		customerSSOSubject := checkoutSession.ClientReferenceID //todo think about maybe not passing this around in plaintext -- couldnt i send my server signed jwt?
		log.Info().Msgf("checkout.session.completed customerSSOSubject: %v", customerSSOSubject)
		log.Info().Msgf("checkout.session.completed checkoutSession.AmountTotal: %v", checkoutSession.AmountTotal)
		log.Info().Msgf("checkout.session.completed checkoutSession.PaymentStatus: %v", checkoutSession.PaymentStatus)

		err = s.provisionAPIKeyForUser(r.Context(), checkoutSession.ID, customerSSOSubject, totalCreditsToIssue)
		if err != nil {
			log.Error().Msgf("error from provisionAPIKeyForUser: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	// ... handle other event types ... if i want to keep track of payment intent ids to make them meaningful? i guess ...
	//  too bad checkout.session.complete is the _only_ one that sends along the client_reference_id - the dashboard doesnt even save it. weak saucen?
	default:
		log.Trace().Msgf("Unhandled event type: %s\n", event.Type)
		w.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
	}

	//log.Trace().Msgf("webhook handler so sleepy ...")
	//time.Sleep(10 * time.Second)
	//
	//http.Error(w, "server went rogue", http.StatusInternalServerError)
}

// Generates a random alphanumeric string of length n
func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[rand.Intn(62)]
	}
	return string(b)
}

var ErrObjectAlreadyExists = errors.New("object already exists")

// Idempotency check for the checkout session
func (s *pdfInspectorServer) idempotencyCheck(ctx context.Context, checkoutSessionID, apiKey string) error {
	gcsFs, ok := s.jobRunner.Tuner.Fs.(*filesystem.GCSFileSystem)
	if !ok {
		log.Error().Msg("s.Fs is not of type *GCSFilesystem")
		return errors.New("couldn't get GCS client")
	}

	client := gcsFs.Client
	bucket := client.Bucket(s.config.GcsBucket)

	// Path for idempotency check
	idempotencyPath := fmt.Sprintf("paymentcs/%s", checkoutSessionID)
	obj := bucket.Object(idempotencyPath)

	// Set precondition to ensure the object is only created if it does not exist
	wc := obj.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	if _, err := wc.Write([]byte(apiKey)); err != nil {
		return fmt.Errorf("failed to write to object: %v", err)
	}

	// Handle the error on Close() since the precondition failure happens here
	if err := wc.Close(); err != nil {
		if strings.Contains(err.Error(), "conditionNotMet") {
			log.Error().Msg("Object already exists, bailing out due to precondition failure.")
			return ErrObjectAlreadyExists
		}
		return fmt.Errorf("failed to close writer: %v", err)
	}
	return nil
}

// Provisions a new API key by writing to users/{apikey}/credit
func (s *pdfInspectorServer) provisionAPIKeyCredit(ctx context.Context, apiKey string, credits int64) error {
	gcsFs, ok := s.jobRunner.Tuner.Fs.(*filesystem.GCSFileSystem)
	if !ok {
		log.Error().Msg("s.Fs is not of type *GCSFilesystem")
		return errors.New("couldn't get GCS client")
	}

	client := gcsFs.Client
	bucket := client.Bucket(s.config.GcsBucket)

	// Path for the credit file
	creditFilePath := fmt.Sprintf("users/%s/credit", apiKey)
	creditObj := bucket.Object(creditFilePath)

	// Write the number of credits to the object
	wc := creditObj.NewWriter(ctx)
	wc.ContentType = "text/plain"
	if _, err := wc.Write([]byte(fmt.Sprintf("%d", credits))); err != nil {
		return fmt.Errorf("failed to write credits to GCS: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close credit file writer: %v", err)
	}

	log.Info().Msgf("Provisioned API key %s with %d credits", apiKey, credits)
	return nil
}

// Appends the new API key to the user's list of API keys at sso/{ssoID}/apikeys
func (s *pdfInspectorServer) addAPIKeyToUser(ctx context.Context, ssoID string, apiKey string) error {
	gcsFs, ok := s.jobRunner.Tuner.Fs.(*filesystem.GCSFileSystem)
	if !ok {
		log.Error().Msg("s.Fs is not of type *GCSFilesystem")
		return errors.New("couldn't get GCS client")
	}

	client := gcsFs.Client
	bucket := client.Bucket(s.config.GcsBucket)

	// Path for the API key list file
	apiKeyPath := fmt.Sprintf("sso/%s/apikeys", ssoID)
	apiKeyObj := bucket.Object(apiKeyPath)

	// Read the existing contents of the API key file if it exists
	_, err := apiKeyObj.Attrs(ctx)
	var existingKeys []string
	if errors.Is(storage.ErrObjectNotExist, err) {
		// No existing API key file, we will create a new one
		existingKeys = []string{}
	} else if err == nil {
		// File exists, read the content
		rc, err := apiKeyObj.NewReader(ctx)
		if err != nil {
			return fmt.Errorf("failed to read existing api keys: %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return fmt.Errorf("failed to read object data: %v", err)
		}

		// Split the file contents into lines (API keys), trim any empty entries
		existingKeys = strings.Split(strings.TrimSpace(string(data)), "\n")
	} else {
		return fmt.Errorf("failed to check object attributes: %v", err)
	}

	// Append the new API key to the list of existing keys
	existingKeys = append(existingKeys, apiKey)

	// Write back the updated API key file
	wc := apiKeyObj.NewWriter(ctx)
	wc.ContentType = "text/plain"
	if _, err := wc.Write([]byte(strings.Join(existingKeys, "\n"))); err != nil {
		return fmt.Errorf("failed to write updated api keys: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close api key writer: %v", err)
	}

	log.Info().Msgf("Appended new API key %s to sso user %s", apiKey, ssoID)
	return nil
}

// Main function to provision API key for a user
func (s *pdfInspectorServer) provisionAPIKeyForUser(ctx context.Context, checkoutSessionID string, ssoID string, credits int64) error {

	// Step 1: Generate API key
	apiKey := randomString(64)

	// Step 2: Idempotency check
	err := s.idempotencyCheck(ctx, checkoutSessionID, apiKey)
	if errors.Is(err, ErrObjectAlreadyExists) {
		// If the object already exists, silently return
		log.Info().Msgf("Checkout session %s already provisioned, skipping.", checkoutSessionID)
		return nil
	} else if err != nil {
		return fmt.Errorf("idempotency check failed: %v", err)
	}

	// Step 3: Provision the API key credit file
	if err := s.provisionAPIKeyCredit(ctx, apiKey, credits); err != nil {
		return fmt.Errorf("failed to provision API key credit: %v", err)
	}

	// Step 4: Add the API key to the user's list
	if err := s.addAPIKeyToUser(ctx, ssoID, apiKey); err != nil {
		return fmt.Errorf("failed to append API key to user: %v", err)
	}

	return nil
}
