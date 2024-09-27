package config

import (
	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"context"
	"flag"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
)

// ServiceConfig struct to hold the configuration values
type ServiceConfig struct {
	GotenbergURL         string
	JsonServerURL        string
	ReactAppURL          string
	FsType               string
	Mode                 string
	LocalPath            string
	GcsBucket            string
	OpenAiApiKey         string //oh noes the capitalization *hand waving* guess what? idgaf :) my way.
	ServiceUrl           string
	UseSystemGs          bool //in the deployed environment we will bake a gs into the image that runs this part, so we can just use a 'gs' command locally.
	ServiceListenPort    string
	AdminKey             string
	UserCreditDeduct     int
	LogLevel             int
	FrontendClientID     string
	FrontendClientSecret string
	JwtSecret            string
}

func InitLogging() int {
	logLevel := getConfigInt(nil, "LOG_LEVEL", 1) //1=Info (-1=Trace,0=Debug,2=Warn,3=Error,etc, see the top of zerolog/log.go)
	zerolog.SetGlobalLevel(zerolog.Level(logLevel))
	return logLevel
}

// GetServiceConfig function to return a pointer to serviceConfig
func GetServiceConfig(logLevel int) *ServiceConfig {
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

	// Populate the serviceConfig struct
	config := &ServiceConfig{
		GotenbergURL:         getConfig(gotenbergURL, "GOTENBERG_URL", "http://localhost:80"),
		JsonServerURL:        getConfig(jsonServerURL, "JSON_SERVER_URL", "http://localhost:3002"),
		ReactAppURL:          getConfig(reactAppURL, "REACT_APP_URL", "http://host.docker.internal:3000"),
		OpenAiApiKey:         getConfig(openAiApiKey, "OPENAI_API_KEY", ""),
		FsType:               getConfig(fstype, "FSTYPE", "local"),
		GcsBucket:            getConfig(gcsBucket, "GCS_BUCKET", "my-stinky-bucket"),
		LocalPath:            getConfig(localPath, "LOCAL_PATH", "outputs"),
		Mode:                 getConfig(mode, "MODE", "server"), // Default to "server"
		UseSystemGs:          getConfigBool(useSystemGs, "USE_SYSTEM_GS", false),
		AdminKey:             getConfig(nil, "ADMIN_KEY", ""),
		UserCreditDeduct:     getConfigInt(nil, "USER_CREDIT_DEDUCT", 1),
		LogLevel:             logLevel,
		FrontendClientID:     getConfig(nil, "FRONTEND_SSO_CLIENT_ID", ""),
		FrontendClientSecret: getConfig(nil, "FRONTEND_SSO_CLIENT_SECRET", ""),
		JwtSecret:            getConfig(nil, "JWT_SECRET", ""), //todo make sure this gets put into secrets and set in the deploy.
	}

	//Validation
	if config.FsType == "gcs" && config.GcsBucket == "" {
		log.Fatal().Msg("GCS bucket name must be specified for GCS filesystem")
	}

	if config.FsType == "local" && config.LocalPath == "" {
		log.Fatal().Msg("Local path must be specified for local filesystem")
	}
	if config.Mode == "server" {
		if config.OpenAiApiKey == "" {
			log.Fatal().Msg("An Open AI (what a misnomer lol) API Key is required for the server to be able to do anything interesting.")
		}

		//todo astute-backup-434623-h3 at least should probably be an env var. or ... idk a robot suggested GOOGLE_CLOUD_PROJECT in the env might have it
		url, err := getServiceURL("astute-backup-434623-h3", "us-central1", "pdfinspector")
		if err != nil {
			//not fatal. might not have credentials to access this.
			log.Error().Msgf("failed to get service URL : %s", err.Error())
		}
		config.ServiceUrl = url
		log.Info().Msgf("determined service url to be: %s", url)

		config.ServiceListenPort = "8080"
	}

	return config
}

// todo: investigate if we can try out that generic stuff so that i dont have to have 2 versions of a function one for string and one for bool.
// Helper function to get value from CLI args, env vars, or default
func getConfig(cliValue *string, envVar string, defaultValue string) string {
	if cliValue != nil && *cliValue != "" {
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
func getConfigInt(cliValue *int, envVar string, defaultValue int) int {
	// First, check if the CLI value is provided (and non-zero, assuming 0 means no value provided)
	if cliValue != nil && *cliValue != 0 {
		return *cliValue
	} else if envVal, exists := os.LookupEnv(envVar); exists {
		// Otherwise, check if the environment variable exists and is parseable as an integer
		parsedValue, err := strconv.Atoi(envVal)
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

	//client, err := run.NewServicesClient(ctx, option.WithCredentialsFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
	client, err := run.NewServicesClient(ctx)
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
