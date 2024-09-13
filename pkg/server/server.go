package server

import (
	"bufio"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/filesystem"
	"pdfinspector/pkg/job"
	"pdfinspector/pkg/jobrunner"
	"pdfinspector/pkg/tuner"
	"strconv"
	"strings"
	"time"
)

type pdfInspectorServer struct {
	config    *config.ServiceConfig
	router    *chi.Mux
	jobRunner *jobrunner.JobRunner
	userKeys  map[string]bool // To store the loaded user API keys
}

func NewPdfInspectorServer(config *config.ServiceConfig) *pdfInspectorServer {
	server := &pdfInspectorServer{
		config: config,
		jobRunner: &jobrunner.JobRunner{
			Config: config,
			Tuner:  tuner.NewTuner(config),
		},
	}
	server.initRoutes()
	server.LoadUserKeys()
	return server
}

func (s *pdfInspectorServer) initRoutes() {
	router := chi.NewRouter()

	// Add middleware
	//router.Use(middleware.Logger)                    // Log requests
	router.Use(structuredLogger)                     // Log requests ... better
	router.Use(middleware.Recoverer)                 // Recover from panics
	router.Use(middleware.Timeout(15 * time.Minute)) // Set a request timeout

	// Define open routes
	router.Get("/", s.rootHandler)                 // Root handler
	router.Get("/health", s.healthHandler)         // Health check handler
	router.Get("/joboutput/*", s.jobOutputHandler) // Get the output

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

	inputJob.PrepareDefault()

	if isAdmin, _ := r.Context().Value("isAdmin").(bool); isAdmin {
		inputJob.IsForAdmin = true
	} else {
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

	data, err := s.jobRunner.Tuner.Fs.ReadFile(resultPath)
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

		// Check if the token is a known user by checking the UserKeys map
		if _, ok := s.userKeys[bearerToken]; !ok {
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

// LoadUserKeys reads all files from the "users/" directory that match "list*.txt"
// and reads newline-separated API keys into the UserKeys array.
func (s *pdfInspectorServer) LoadUserKeys() {
	// Define the directory and file pattern
	dir := "users/"
	pattern := "list*.txt"

	// Use filepath.Glob to find all matching files
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		log.Info().Msgf("Error finding files in %s: %v", dir, err)
		return
	}

	// If no files match the pattern, log it and return
	if len(files) == 0 {
		log.Info().Msgf("No files matching pattern %s found in directory %s", pattern, dir)
		return
	}

	s.userKeys = make(map[string]bool)

	// Iterate over each file
	for _, file := range files {
		// Open the file for reading
		f, err := os.Open(file)
		if err != nil {
			log.Error().Msgf("Error opening file %s: %v", file, err)
			continue
		}
		defer f.Close()

		// Use bufio.Scanner to read the file line by line
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			// Each line is an API key, add it to UserKeys
			apiKey := strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				s.userKeys[apiKey] = true
			}
		}

		// Log any scanning errors (such as malformed input)
		if err := scanner.Err(); err != nil {
			log.Error().Msgf("Error reading file %s: %v", file, err)
		}
	}

	log.Info().Msgf("Loaded %d user keys", len(s.userKeys))
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
