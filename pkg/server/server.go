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
	"google.golang.org/api/idtoken"
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

	// Define open routes
	router.Get("/", s.rootHandler)                 // Root handler
	router.Get("/health", s.healthHandler)         // Health check handler
	router.Get("/joboutput/*", s.jobOutputHandler) // Get the output
	router.Get("/schema/{layout}", s.GetExpectedResponseJsonSchemaHandler)
	router.Get("/getapitoken", s.GetAPIToken)

	// Define gated routes
	router.Group(func(protected chi.Router) {
		protected.Use(s.AuthMiddleware)
		protected.Post("/streamjob", s.streamJobHandler) // Keep the connection open while running the job and streaming updates
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
	log.Trace().Msg("here in streamJobHandler")

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
			http.Error(w, "Bad Reqeust: invalid inputJob", http.StatusBadRequest)
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

// AuthMiddleware is the middleware that checks for a valid Bearer token and session information in GCS.
func (s *pdfInspectorServer) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: No authorization header provided", http.StatusUnauthorized)
			return
		}

		// Expect a Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
			http.Error(w, "Unauthorized: Malformed authorization header", http.StatusUnauthorized)
			return
		}

		bearerToken := tokenParts[1]

		// Check if the bearer token matches the AdminKey in the config
		if bearerToken == s.config.AdminKey {
			// Allow the request through if it's an admin token
			// Set the admin flag in the context
			ctx := context.WithValue(r.Context(), "isAdmin", true)

			// Create a new request with the updated context
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
			return
		}

		knownApiKey, err := s.checkApiKeyExists(r.Context(), bearerToken)
		if err != nil || knownApiKey == false {
			// If the token is not a known user, deny access
			http.Error(w, "Unauthorized: Unknown user token", http.StatusUnauthorized)
			return
		}

		// If everything is valid, proceed to the next handler
		ctx := context.WithValue(r.Context(), "userKey", bearerToken)

		// Create a new request with the updated context
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// Method to check if an API key exists using GCS and cache results.
func (s *pdfInspectorServer) checkApiKeyExists(ctx context.Context, apiKey string) (bool, error) {
	// 1. First, check the map (knownUsers) for the API key.
	s.userKeysMu.RLock()
	actualApiKey, hasRecordOfChecking := s.userKeys[apiKey]
	s.userKeysMu.RUnlock()

	// If the API key is found in the cache, return the cached result.
	if hasRecordOfChecking {
		return actualApiKey, nil
	}

	// 2. If the API key is not found in the cache, check GCS.
	// First, check if s.jobRunner.Tuner.Fs is a GcpFilesystem.
	gcpFs, ok := s.jobRunner.Tuner.Fs.(*filesystem.GCSFileSystem)
	if !ok {
		return false, fmt.Errorf("file system is not GCSFileSystem")
	}

	// Use the GCS client to check for the file's existence.
	client := gcpFs.Client
	bucketName := s.config.GcsBucket
	objectName := fmt.Sprintf("users/%s/credit", apiKey) // Assuming the API keys are stored in a "keys" folder

	// 3. Check if the API key file exists in GCS.
	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	_, err := obj.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		// The API key file does not exist, cache the result as false.
		log.Info().Msgf("Discovered and cached the fact that api key %s does _not_ exist.", apiKey)
		s.userKeysMu.Lock()
		s.userKeys[apiKey] = false
		s.userKeysMu.Unlock()
		return false, nil
	} else if err != nil {
		// Some other error occurred.
		return false, fmt.Errorf("error checking API key in GCS: %v", err)
	}
	log.Info().Msgf("Discovered and cached the fact that api key %s exists.", apiKey)
	// 4. If the file exists, cache the result as true.
	s.userKeysMu.Lock()
	s.userKeys[apiKey] = true
	s.userKeysMu.Unlock()
	return true, nil
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

func (s *pdfInspectorServer) GetExpectedResponseJsonSchemaHandler(w http.ResponseWriter, r *http.Request) {
	layout := chi.URLParam(r, "layout")
	log.Info().Msgf("here in GetExpectedResponseJsonSchemaHandler for %s", layout)
	schema, err := s.jobRunner.Tuner.GetExpectedResponseJsonSchema(layout)
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

// GetAPIToken handles the verification of the Google ID token sent from the client.
func (s *pdfInspectorServer) GetAPIToken(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("here in getAPItoken")
	if s.config.FrontendClientID == "" {
		//because if this is blank and then we just try to validate against a blank audience then it will accept any token i think regardless of audience or where it came from.
		http.Error(w, "Error: Misconfigured server", http.StatusInternalServerError)
		return
	}

	credential := r.Header.Get("X-Credential")
	if credential == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	payload, err := idtoken.Validate(r.Context(), credential, s.config.FrontendClientID)
	if err != nil {
		http.Error(w, "Invalid ID token", http.StatusUnauthorized)
		return
	}

	// Step 3: Extract user identifier (e.g., email, sub) from the payload
	userID := payload.Subject // The unique user ID (sub claim)
	email, ok := payload.Claims["email"].(string)
	log.Info().Msgf("we thinks %s just logged in, %#v", userID, payload)
	if !ok || email == "" {
		http.Error(w, "Email not found in token", http.StatusUnauthorized)
		return
	}

	apiKey, err := s.GetBestApiKeyForUser(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not determine best APIKey: %s", err.Error()), http.StatusUnauthorized)
	}

	// Step 5: Return the API key as a JSON response
	w.Header().Set("Content-Type", "text/plain")
	//response := map[string]string{"apiKey": apiKey}
	//json.NewEncoder(w).Encode(response)
	w.Write([]byte(apiKey))
}

// GetBestApiKeyForUser retrieves the best API key for a user based on the least positive remaining credits.
func (s *pdfInspectorServer) GetBestApiKeyForUser(ctx context.Context, userID string) (string, error) {
	// Step 1: Read the API keys for the user
	apiKeys, err := s.ReadApiKeysForUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to read API keys for user %s: %w", userID, err)
	}
	if len(apiKeys) == 0 {
		return "", fmt.Errorf("no API keys found for user %s", userID)
	}

	// Step 2: For each API key, get the credit and find the one with the least positive credits
	var bestApiKey string
	minCredits := math.MaxInt64 // Initialize to maximum int value
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
		if credits < minCredits {
			minCredits = credits
			bestApiKey = apiKey
		}
	}

	if bestApiKey == "" {
		return "", fmt.Errorf("no API keys with positive credits found for user %s", userID)
	}

	// Return the best API key
	log.Printf("Selected API key %s with %d credits for user %s", bestApiKey, minCredits, userID)
	return bestApiKey, nil
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
