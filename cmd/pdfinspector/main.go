package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/job"
	"pdfinspector/pkg/server"
	"pdfinspector/pkg/tuner"
)

// main function: Either runs the web server or executes the main functionality.
func main() {
	// Get the service configuration
	config := config.GetServiceConfig(config.InitLogging())

	if config.Mode == "cli" {
		// CLI execution mode
		fmt.Println("Running main functionality from CLI...")
		cliRunJob(config)
		fmt.Println("Finished Executing main functionality via CLI")
		return
	}

	// Web server mode
	server := server.NewPdfInspectorServer(config)
	server.RunServer()
}

func cliRunJob(config *config.ServiceConfig) {

	// Example call to ReadInput
	inputDir := "input" // The directory where jd.txt and expect_response.json are located

	//baseline := "functional"
	baseline := "chrono"
	//baseline := "resumeproject"
	//baseline := "retailchrono"
	//outputDir := "outputs"
	styleOverride := "fluffy"
	//styleOverride := ""

	t := tuner.NewTuner(config)
	baselineJSON, err := t.GetBaselineJSON(baseline)
	if err != nil {
		log.Fatal().Msgf("error from reading baseline JSON: %v", err)
	}
	layout, style, err := t.GetLayoutFromBaselineJSON(baselineJSON)
	if err != nil {
		log.Fatal().Msgf("error from extracting layout from baseline JSON: %v", err)
	}
	if styleOverride != "" {
		style = styleOverride
	}

	input, err := job.ReadInput(inputDir)
	if err != nil {
		log.Fatal().Msgf("Error reading input files: %v", err)
	}
	if input.APIKey != "" {
		config.OpenAiApiKey = input.APIKey
	}

	mainPrompt, err := getInputPrompt(inputDir)
	if err != nil {
		log.Error().Msgf("error from reading input prompt: %s", err.Error())
		return
	}
	if mainPrompt == "" {
		mainPrompt, err = t.GetDefaultPrompt(layout)
		if err != nil {
			log.Error().Msgf("error from reading input prompt: %s", err.Error())
			return
		}
	}

	// this func could possibly leverage tuner.PopulateJob but doing that also might be annoying. tbd.
	inputJob := job.NewDefaultJob()
	inputJob.JobDescription = input.JD
	inputJob.Layout = layout
	inputJob.Style = style
	inputJob.MainPrompt = mainPrompt
	inputJob.BaselineJSON = baselineJSON
	//inputJob.OutputDir = filepath.Join(config.LocalPath, inputjob.Id) //should not use this for things that end up on gcs from a windows machine b/c it gets a backslash. idk probably should have local and gcs dirs saved separately so local can use local path sep and gcs always use forward slash.
	inputJob.OutputDir = fmt.Sprintf("%s/%s", config.LocalPath, inputJob.Id)

	err = t.TuneResumeContents(inputJob, nil)
	if err != nil {
		log.Fatal().Msgf("Error from resume tuning: %v", err)
	}
}

func getInputPrompt(directory string) (string, error) {
	// Construct the filename
	filepath := filepath.Join(directory, "prompt.txt")

	// Check if the file exists
	_, err := os.Stat(filepath)
	if err == nil {
		// File exists, read its contents
		data, err := os.ReadFile(filepath)
		if err != nil {
			return "", fmt.Errorf("prompt file existed but failed to read it from file system??%v", err)
		}
		return string(data), nil
	}
	return "", nil
}
