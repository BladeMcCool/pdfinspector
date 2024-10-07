package server

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
	"io"
	"math"
	"mime"
	"net/http"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/filesystem"
	"pdfinspector/pkg/job"
	"pdfinspector/pkg/jobrunner"
	"pdfinspector/pkg/tuner"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pdfInspectorServer struct {
	config     *config.ServiceConfig
	router     *chi.Mux
	jobRunner  *jobrunner.JobRunner
	userKeys   map[string]bool // To store the loaded user API keys
	userKeysMu sync.RWMutex
}

func NewPdfInspectorServer(config *config.ServiceConfig) *pdfInspectorServer {
	server := &pdfInspectorServer{
		config: config,
		jobRunner: &jobrunner.JobRunner{
			Config: config,
			Tuner:  tuner.NewTuner(config),
		},
		userKeys: map[string]bool{},
	}
	server.initRoutes()
	return server
}

func (s *pdfInspectorServer) initRoutes() {
	router := chi.NewRouter()

	// Add middleware
	//router.Use(middleware.Logger)                    // Log requests
	router.Use(structuredLogger)                     // Log requests ... better
	router.Use(middleware.Recoverer)                 // Recover from panics
	router.Use(middleware.Timeout(15 * time.Minute)) // Set a request timeout

	// CORS middleware configuration (thanks ChatGPT!)
	router.Use(cors.Handler(cors.Options{
		// Allow all origins, or specify the ones you need
		AllowedOrigins: []string{"*"},
		// Allow all methods
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		// Allow all headers, including the Authorization header
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Credential"},
		// Allow credentials
		AllowCredentials: true,
		// Set max preflight age to avoid repeated preflight requests (in seconds)
		MaxAge: 300,
	}))

	router.Use(s.SSOUserDetectionMiddleware)

	// Define open routes
	router.Get("/", s.rootHandler)                 // Root handler
	router.Get("/health", s.healthHandler)         // Health check handler
	router.Get("/joboutput/*", s.jobOutputHandler) // Get the output
	router.Get("/schema/{layout}", s.GetJsonSchemaHandler)
	router.Get("/getapitoken", s.GetAPIToken)
	router.Get("/getusergenids", s.GetUserGenIDsHandler)
	//router.Post("/create-payment-intent", s.handleCreatePaymentIntent)
	router.Post("/stripe-webhook", s.handleStripeWebhook)

	// Define gated routes
	router.Group(func(protected chi.Router) {
		protected.Use(s.AuthMiddleware)
		protected.Post("/streamjob", s.streamJobHandler) // Keep the connection open while running the job and streaming updates
		protected.Post("/extractresumedata/{layout}", s.extractResumeHandler)

		//template CRUD
		protected.Get("/templates", s.ListTemplatesHandler)
		protected.Post("/templates", s.CreateTemplateHandler)
		protected.Get("/templates/{template}", s.ReadTemplateHandler)
		protected.Put("/templates/{template}", s.UpdateTemplateHandler)
		protected.Delete("/templates/{template}", s.DeleteTemplateHandler)
	})

	s.router = router
}

// RunServer starts the HTTP server and listens for requests
func (s *pdfInspectorServer) RunServer() error {
	addr := fmt.Sprintf(":%s", s.config.ServiceListenPort)
	log.Info().Msgf("Starting server on port %s...", s.config.ServiceListenPort)

	// Start the HTTP server with the chi router
	return http.ListenAndServe(addr, s.router)
}

func structuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Call the next handler
		next.ServeHTTP(w, r)
		// After the response
		log.Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Str("remote_addr", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("HTTP request completed")
	})
}

// Handler for the root path
func (s *pdfInspectorServer) rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to pdf inspector.!")
	//maybe for sake of ease we can spit out a job inputter utility page but probably that should just be a separate project in react or smth instead of some horrifying cheese here. but maybe gpt can help me cheese it up quickly. lets see.
}

// Handler for a health check endpoint
func (s *pdfInspectorServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *pdfInspectorServer) streamJobHandler(w http.ResponseWriter, r *http.Request) {
	var inputJob job.Job
	if err := json.NewDecoder(r.Body).Decode(&inputJob); err != nil {
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}

	if isAdmin, _ := r.Context().Value("isAdmin").(bool); isAdmin {
		inputJob.PrepareDefault(inputJob.OverrideJobId)
		inputJob.IsForAdmin = true
	} else {
		inputJob.PrepareDefault(nil)
		err := inputJob.ValidateForNonAdmin()
		if err != nil {
			log.Error().Msgf("invalid inputJob %v", err)
			http.Error(w, "Bad Request: invalid inputJob", http.StatusBadRequest)
			return
		}
		userKey, _ := r.Context().Value("userKey").(string)
		//todo: if the inputJob fails return credit.
		err, inputJob.UserCreditRemaining = s.deductUserCredit(r.Context(), userKey)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		inputJob.UserKey = userKey

		userID, _ := r.Context().Value("ssoSubject").(string)
		inputJob.UserID = userID
		log.Trace().Msgf("streamJobHandler: sso subject userId believed to be %s", inputJob.UserID)
	}

	// Set headers for streaming response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	// Create a channel to communicate inputJob status updates
	// Stream status updates to the client
	var encounteredError = false
	for status := range s.jobRunner.RunJobStreaming(&inputJob) {
		// Create a JobStatus struct with the status message

		// Marshal the status update to JSON
		data, err := json.Marshal(status)
		if err != nil {
			http.Error(w, "Error encoding status", http.StatusInternalServerError)
			return
		}

		// Write the JSON status update to the response
		if status.Error != nil {
			encounteredError = true
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		if err != nil {
			log.Debug().Msg("Client connection lost.")
			return
		}

		// Flush the response writer to ensure the data is sent immediately
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	var finalResult interface{}
	if encounteredError {
		// Final result after inputJob non completion
		finalResult = job.JobResult{
			Status:  "Failed",
			Details: "The inputJob failed with an error.",
		}
	} else {
		// Final result after inputJob completion
		finalResult = job.JobResult{
			Status:  "Completed",
			Details: "The inputJob was successfully completed.",
		}
	}

	// Marshal the final result to JSON
	finalData, err := json.Marshal(finalResult)
	if err != nil {
		http.Error(w, "Error encoding final result", http.StatusInternalServerError)
		return
	}

	// Send the final JSON result to the client
	fmt.Fprintf(w, "%s\n", finalData)

	// Flush final response to the client
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *pdfInspectorServer) jobOutputHandler(w http.ResponseWriter, r *http.Request) {
	//todo can we update this for path params now that we are using chi?
	// Extract the path and split it by '/'
	pathParts := strings.Split(r.URL.Path, "/")
	// should become stuff like []string{"","jobresult","somejobid","somepdf"}
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Rejoin everything after "/jobresult/"
	// pathParts[2:] contains everything after "/jobresult/"
	resultPath := strings.Join(append([]string{"outputs"}, pathParts[2:]...), "/")

	// Use the rejoined path as needed
	log.Info().Msgf("Result Path: %s", resultPath)

	data, err := s.jobRunner.Tuner.Fs.ReadFile(r.Context(), resultPath)
	if err != nil {
		http.Error(w, "Could not read file from GCS", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("Read %d bytes of data from GCS", len(data))

	// Infer the Content-Type based on the file extension
	fileName := pathParts[len(pathParts)-1]
	log.Info().Msgf("Send back with filename: %s", fileName)
	ext := filepath.Ext(fileName)
	mimeType := mime.TypeByExtension(ext)

	// Fallback to application/octet-stream if we can't determine the MIME type
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Set the headers
	w.Header().Set("Content-Type", mimeType)
	disposition := "attachment"
	if _, ok := r.URL.Query()["inline"]; ok {
		// Set Content-Disposition to "inline" to display in the browser
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%s", disposition, fileName))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	// Write the PDF content to the response
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, "Unable to write file to response", http.StatusInternalServerError)
		return
	}
}

func (s *pdfInspectorServer) deductUserCredit(ctx context.Context, userKey string) (error, int) {
	//this is really just a best effort to create some kind of locking mechanism with gcs in the absence of anything stateful
	//because i dont want to pay for a "real" solution (eg hosted database record locking or smth)

	// Path to the user's credit file in GCS
	creditFilePath := fmt.Sprintf("users/%s/credit", userKey)
	_ = creditFilePath
	// Step 1: Get the generation number of the credit file
	gcsFs, ok := s.jobRunner.Tuner.Fs.(*filesystem.GCSFileSystem)
	_ = gcsFs
	if !ok {
		// Handle the error if the type assertion fails
		log.Error().Msg("s.Fs is not of type *GCSFilesystem")
		return errors.New("couldnt get gcs client"), 0
	}
	//
	client := gcsFs.Client
	_ = client

	attrs, err := client.Bucket(s.config.GcsBucket).Object(creditFilePath).Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get generation number: %w", err), 0
	}

	// Step 2: Read the current credit balance
	rc, err := client.Bucket(s.config.GcsBucket).Object(creditFilePath).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to read credit file: %w", err), 0
	}
	defer rc.Close()
	fileData, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read credit file: %w", err), 0
	}

	// Parse the credit data (assuming it's stored as a single integer)
	currentCredit, err := strconv.Atoi(strings.TrimSpace(string(fileData)))
	if err != nil {
		return fmt.Errorf("invalid credit format: %w", err), 0
	}
	//
	log.Info().Msgf("user %s has %d credit", userKey, currentCredit)
	deductionAmount := s.config.UserCreditDeduct
	// Step 3: Check if the user has enough credit
	if currentCredit-deductionAmount < 0 {
		// Deny the request if doing so would put us into negative balance
		return fmt.Errorf("insufficient credit, request denied"), 0
	}

	// Step 4: Deduct one credit
	newCredit := currentCredit - deductionAmount

	// Prepare the new credit data
	newCreditData := []byte(fmt.Sprintf("%d", newCredit))
	wc := client.Bucket(s.config.GcsBucket).Object(creditFilePath).If(storage.Conditions{GenerationMatch: attrs.Generation}).NewWriter(ctx)
	defer wc.Close()
	if _, err := wc.Write(newCreditData); err != nil {
		return fmt.Errorf("failed to deduct credit, possible concurrent modification: %w", err), 0
	}

	// Credit deduction successful
	return nil, newCredit
}

func (s *pdfInspectorServer) GetJsonSchemaHandler(w http.ResponseWriter, r *http.Request) {
	layout := chi.URLParam(r, "layout")
	log.Info().Msgf("here in GetJsonSchemaHandler for %s", layout)
	schema, err := s.jobRunner.Tuner.GetCompleteJsonSchema(layout)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Here is how we would add some random key of info to the schema before we serialize it back and send it out
	//if schemaMap, ok := schema.(map[string]interface{}); ok {
	//	schemaMap["additionalInfo"] = "Some extra info"
	//}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(schema); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// TODO rename this stuff to apikey
// GetAPIToken handles the verification of the Google ID token sent from the client.
func (s *pdfInspectorServer) GetAPIToken(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("here in getAPItoken")

	userID, _ := r.Context().Value("ssoSubject").(string)
	if userID == "" {
		log.Trace().Msg("ssoSubject/userID was empty?")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	apiKey, totalCredits, err := s.GetBestApiKeyForUser(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not determine best APIKey: %s", err.Error()), http.StatusUnauthorized)
		return
	}

	jwt, err := s.CreateCustomToken(userID) //this is really about being able to verify claims that a apikey is making a generation for a sso sub id, so that we can record the generation there. i dont want to just trust the user and i dont want to deal with google sso oauth2 refresh token management and shit just to be able to validate the sso credential (which expires after 1 hour in the simple auth mode where we just get a signed id token from google api) so we can't rely on that being still 'valid'. however, if we roll our own at a time when we know the sso credential is valid to associate that apikey with that sso sub id, then i think it serves the purpose. thanks for reading!
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Step 5: Return the API key as a JSON response. and a jwt with a better expiry that we can keep refreshing just because.
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"apiKey":       apiKey,
		"totalCredits": totalCredits,
		"jwt":          jwt,
	}
	json.NewEncoder(w).Encode(response)
}

// GetBestApiKeyForUser retrieves the best API key for a user based on the least positive remaining credits.
func (s *pdfInspectorServer) GetBestApiKeyForUser(ctx context.Context, userID string) (string, int, error) {
	// this is all cheese because i'm using GCS as a database.

	// Step 1: Read the API keys for the user
	apiKeys, err := s.ReadApiKeysForUser(ctx, userID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read API keys for user %s: %w", userID, err)
	}
	if len(apiKeys) == 0 {
		return "", 0, fmt.Errorf("no API keys found for user %s", userID)
	}

	// Step 2: For each API key, get the credit and find the one with the least positive credits
	var bestApiKey string
	minCredits := math.MaxInt64 // Initialize to maximum int value
	totalCredits := 0
	for _, apiKey := range apiKeys {
		credits, err := s.GetCreditsForApiKey(ctx, apiKey)
		if err != nil {
			log.Printf("Failed to get credits for API key %s: %v", apiKey, err)
			continue
		}
		if credits <= 0 {
			log.Printf("API key %s has zero or negative credits (%d), skipping", apiKey, credits)
			continue
		}
		totalCredits += credits
		if credits < minCredits {
			minCredits = credits
			bestApiKey = apiKey
		}
	}

	if bestApiKey == "" {
		return "", 0, fmt.Errorf("no API keys with positive credits found for user %s", userID)
	}

	// Return the best API key
	log.Printf("Selected API key %s with %d credits for user %s", bestApiKey, minCredits, userID)
	return bestApiKey, totalCredits, nil
}

// ReadApiKeysForUser reads the API keys for a user from GCS.
func (s *pdfInspectorServer) ReadApiKeysForUser(ctx context.Context, userID string) ([]string, error) {
	data, err := s.jobRunner.Tuner.Fs.ReadFile(ctx, fmt.Sprintf("sso/%s/apikeys", userID))
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("API keys file does not exist for user %s", userID)
		}
		return nil, fmt.Errorf("failed to read API keys file for user %s: %w", userID, err)
	}

	// Split the data into lines and clean up the API keys
	lines := strings.Split(string(data), "\n")
	var apiKeys []string
	for _, line := range lines {
		apiKey := strings.TrimSpace(line)
		if apiKey != "" {
			apiKeys = append(apiKeys, apiKey)
		}
	}
	return apiKeys, nil
}

// GetCreditsForApiKey retrieves the remaining credits for a given API key from GCS.
func (s *pdfInspectorServer) GetCreditsForApiKey(ctx context.Context, apiKey string) (int, error) {
	data, err := s.jobRunner.Tuner.Fs.ReadFile(ctx, fmt.Sprintf("users/%s/credit", apiKey))

	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			log.Printf("Credit file does not exist for API key %s", apiKey)
			return 0, nil // Skip this API key
		}
		return 0, fmt.Errorf("failed to read credit file for API key %s: %w", apiKey, err)
	}

	creditStr := strings.TrimSpace(string(data))
	credits, err := strconv.Atoi(creditStr)
	if err != nil {
		return 0, fmt.Errorf("invalid credit value for API key %s: %w", apiKey, err)
	}
	return credits, nil
}

func (s *pdfInspectorServer) GetUserGenIDsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the userId from the request context - it would have come from ripping into credential header of one sort or another.
	userID, ok := r.Context().Value("ssoSubject").(string)
	if !ok || userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	// Call the function to list objects under the user's gen path
	objectInfos, err := s.ListObjectsWithPrefix(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to list objects: "+err.Error(), http.StatusInternalServerError)
		return
	}

	//todo sort them.
	sorted, err := sortAndSerializeGenerations(objectInfos)
	if err != nil {
		http.Error(w, "Failed to sort/serialize objects: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the list of object names in JSON format
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte(sorted))
	if err != nil {
		http.Error(w, "Failed to write output to client: "+err.Error(), http.StatusInternalServerError)
	}
}
