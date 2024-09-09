package main

import (
	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"context"
	"flag"
	"fmt"
	"google.golang.org/api/option"
	"log"
	"os"
	"strconv"
)

// serviceConfig struct to hold the configuration values
type serviceConfig struct {
	gotenbergURL      string
	jsonServerURL     string
	reactAppURL       string
	fsType            string
	mode              string
	localPath         string
	gcsBucket         string
	openAiApiKey      string //oh noes the capitalization *hand waving* guess what? idgaf :) my way.
	serviceUrl        string
	useSystemGs       bool //in the deployed environment we will bake a gs into the image that runs this part, so we can just use a 'gs' command locally.
	serviceListenPort string
}

// getServiceConfig function to return a pointer to serviceConfig
func getServiceConfig() *serviceConfig {
	// Define CLI flags
	gotenbergURL := flag.String("gotenberg-url", "", "URL for Gotenberg service")
	jsonServerURL := flag.String("json-server-url", "", "URL for JSON server")
	reactAppURL := flag.String("react-app-url", "", "URL for React app")
	gcsBucket := flag.String("gcs-bucket", "", "File system type (local or gcs)")
	openAiApiKey := flag.String("api-key", "", "OpenAI API Key")
	localPath := flag.String("local-path", "", "Mode of the application (server or cli)")
	fstype := flag.String("fstype", "", "File system type (local or gcs)")
	mode := flag.String("mode", "", "Mode of the application (server or cli)")
	useSystemGs := flag.Bool("use-system-gs", false, "Use GhostScript from the system instead of via docker run")

	// Parse CLI flags
	flag.Parse()

	//var useSystemGsEnvVar
	//useSystemGsX, err := strconv.ParseBool(getConfig(useSystemGs, "USE_SYSTEM_GS", "false"))
	//if err == nil {
	//	log.Fatalf("%v", err)
	//}
	//, // Default to "server"

	// Populate the serviceConfig struct
	config := &serviceConfig{
		gotenbergURL:  getConfig(gotenbergURL, "GOTENBERG_URL", "http://localhost:80"),
		jsonServerURL: getConfig(jsonServerURL, "JSON_SERVER_URL", "http://localhost:3002"),
		reactAppURL:   getConfig(reactAppURL, "REACT_APP_URL", "http://host.docker.internal:3000"),
		openAiApiKey:  getConfig(openAiApiKey, "OPENAI_API_KEY", ""),
		fsType:        getConfig(fstype, "FSTYPE", "gcs"),
		gcsBucket:     getConfig(gcsBucket, "GCS_BUCKET", "my-stinky-bucket"),
		localPath:     getConfig(localPath, "LOCAL_PATH", "output"),
		mode:          getConfig(mode, "MODE", "server"), // Default to "server"
		useSystemGs:   getConfigBool(useSystemGs, "USE_SYSTEM_GS", true),
	}

	//Validation
	if config.fsType == "gcs" && config.gcsBucket == "" {
		log.Fatal("GCS bucket name must be specified for GCS filesystem")
	}

	if config.fsType == "local" && config.localPath == "" {
		log.Fatal("Local path must be specified for local filesystem")
	}
	if config.openAiApiKey == "" {
		log.Fatal("An Open AI (what a misnomer lol) API Key is required for the server to be able to do anything interesting.")
	}

	url, err := getServiceURL("astute-backup-434623-h3", "us-central1", "pdfinspector")
	if err != nil {
		//not fatal. might not have credentials to access this.
		log.Printf("failed to get service URL : %s", err.Error())
	}
	config.serviceUrl = url
	log.Printf("determined service url to be: %s", url)

	config.serviceListenPort = "8080"

	return config
}

// todo: investigate if we can try out that generic stuff so that i dont have to have 2 versions of a function one for string and one for bool.
// Helper function to get value from CLI args, env vars, or default
func getConfig(cliValue *string, envVar string, defaultValue string) string {
	if *cliValue != "" {
		return *cliValue
	}
	if value, exists := os.LookupEnv(envVar); exists {
		return value
	}
	return defaultValue
}
func getConfigBool(cliValue *bool, envVar string, defaultValue bool) bool {
	// First, check if the CLI value is provided
	if *cliValue {
		return *cliValue
	} else if envVal, exists := os.LookupEnv(envVar); exists {
		// Otherwise, check if the environment variable exists and is parseable as a bool
		parsedValue, err := strconv.ParseBool(envVal)
		if err != nil {
			return defaultValue
		}
		return parsedValue
	}

	// If neither is provided, return the default value
	return defaultValue
}

func getServiceURL(projectID, location, serviceName string) (string, error) {
	// Create a context
	ctx := context.Background()
	_ = ctx

	client, err := run.NewServicesClient(ctx, option.WithCredentialsFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
	// Initialize the Cloud Run client with automatic authentication
	if err != nil {
		return "", fmt.Errorf("failed to create cloud run client: %v", err)
	}
	defer client.Close()
	//
	//// Construct the request to get the service details
	req := &runpb.GetServiceRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, location, serviceName),
	}
	//
	//// Make the API call to get the service
	service, err := client.GetService(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get service: %v", err)
	}
	//
	//// Extract the status URL
	statusURL := service.GetUri()
	if statusURL == "" {
		return "", fmt.Errorf("service URL is not available")
	}
	//
	return statusURL, nil
}
