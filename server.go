package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type pdfInspectorServer struct {
	config *serviceConfig
	router *chi.Mux
}

func newPdfInspectorServer(config *serviceConfig) *pdfInspectorServer {
	server := &pdfInspectorServer{
		config: config,
	}
	server.initRoutes()
	return server
}

func (s *pdfInspectorServer) initRoutes() {
	router := chi.NewRouter()

	// Add middleware
	router.Use(middleware.Logger)                    // Log requests
	router.Use(middleware.Recoverer)                 // Recover from panics
	router.Use(middleware.Timeout(60 * time.Second)) // Set a request timeout

	// Define routes
	router.Get("/", s.rootHandler)                // Root handler
	router.Get("/health", s.healthHandler)        // Health check handler
	router.Post("/streamjob", s.streamJobHandler) // Keep the connection open while running the job and streaming updates
	router.Get("/joboutput", s.jobOutputHandler)  // Get the output

	s.router = router
}

// RunServer starts the HTTP server and listens for requests
func (s *pdfInspectorServer) RunServer() error {
	addr := fmt.Sprintf(":%s", s.config.serviceListenPort)
	log.Printf("Starting server on port %s...\n", s.config.serviceListenPort)

	// Start the HTTP server with the chi router
	return http.ListenAndServe(addr, s.router)
}

// Handler for the root path
func (s *pdfInspectorServer) rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to pdf inspector.!")
}

// Handler for a health check endpoint
func (s *pdfInspectorServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *pdfInspectorServer) streamJobHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("here in streamJobHandler")

	var job Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}
	log.Printf("here in handJobRun with job like: %#v", job)

	// Set headers for streaming response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	// Create a channel to communicate job status updates
	statusChan := make(chan JobStatus)

	//// Add job to queue
	job.Id = uuid.New()
	//how about we call to a job runner? which can be a server property and have a tuner already set in it.
	go runJob(&job, s.config, statusChan)

	// Stream status updates to the client
	for status := range statusChan {
		// Create a JobStatus struct with the status message

		// Marshal the status update to JSON
		data, err := json.Marshal(status)
		if err != nil {
			http.Error(w, "Error encoding status", http.StatusInternalServerError)
			return
		}

		// Write the JSON status update to the response
		_, err = fmt.Fprintf(w, "%s\n", data)
		if err != nil {
			log.Println("Client connection lost.")
			return
		}

		// Flush the response writer to ensure the data is sent immediately
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Final result after job completion
	finalResult := Result{
		Status:  "Completed",
		Details: "The job was successfully completed.",
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
	log.Printf("here 1 ...")
	// Extract the path and split it by '/'
	pathParts := strings.Split(r.URL.Path, "/")
	// should become stuff like []string{"","jobresult","somejobid","somepdf"}
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Rejoin everything after "/jobresult/"
	// pathParts[2:] contains everything after "/jobresult/"
	resultPath := strings.Join(pathParts[2:], "/")

	// Use the rejoined path as needed
	log.Printf("Result Path: %s\n", resultPath)

	data, err := fs.ReadFile(resultPath)
	if err != nil {
		http.Error(w, "Could not read file from GCS", http.StatusBadRequest)
		return
	}
	log.Printf("Read %d bytes of data from GCS", len(data))

	// Infer the Content-Type based on the file extension
	fileName := pathParts[len(pathParts)-1]
	log.Printf("Send back with filename: %s\n", fileName)
	ext := filepath.Ext(fileName)
	mimeType := mime.TypeByExtension(ext)

	// Fallback to application/octet-stream if we can't determine the MIME type
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Set the headers
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	// Write the PDF content to the response
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, "Unable to write file to response", http.StatusInternalServerError)
		return
	}
}
