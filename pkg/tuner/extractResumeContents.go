package tuner

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"pdfinspector/pkg/job"
	"strings"
)

type ResumeExtractResult struct {
	ResumeJSONRaw  string
	ExpectedSchema interface{}
}
type ResumeExtractionJob struct {
	FileContent   []byte
	extractedText string
	Layout        string
	UseSystemGs   bool
	UserID        string
}

const MIN_ACCEPTABLE_RATIO = float64(0.9)
const MAX_ACCEPTABLE_RATIO = float64(1.1)

func (t *Tuner) ExtractResumeContents(job *ResumeExtractionJob, updates chan job.JobStatus) (*ResumeExtractResult, error) {
	SendJobUpdate(updates, "getting idk")
	expectResponseSchema, err := t.GetExpectedResponseJsonSchema(job.Layout)
	if err != nil {
		return nil, fmt.Errorf("Error getting schema: %v\n", err)
	}

	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Error getting current directory: %v\n", err)
	}

	// Construct the output directory path
	extractionId := uuid.New().String()
	outputDirFullpath := filepath.Join(currentDir, "extraction", extractionId)
	err = os.MkdirAll(outputDirFullpath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	//write the file data to a temp file so we can use gs to extract it.
	// to a file called input.pdf in the directory of outputDirFullpath
	inputFilePath := filepath.Join(outputDirFullpath, "input.pdf")

	// Write the file data to the new file (input.pdf)
	err = os.WriteFile(inputFilePath, job.FileContent, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("couldnt write pdf to filesystem")
	}

	//could maybe check the pdf for not containing error stuff like "Uncaught runtime errors" before proceeding.
	// MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd)/output:/workspace minidocks/ghostscript:latest gs -sDEVICE=pngalpha -o /workspace/out-%03d.png -r144 /workspace/attempt.pdf
	var cmd *exec.Cmd
	if job.UseSystemGs {
		cmd = exec.Command(
			"gs",
			"-sDEVICE=txtwrite",
			"-o", filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"),
			filepath.Join(outputDirFullpath, "input.pdf"),
		)
	} else {
		cmd = exec.Command("docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/workspace", outputDirFullpath),
			"minidocks/ghostscript:latest",
			"gs",
			"-sDEVICE=txtwrite",
			"-o", "/workspace/pdf-txtwrite.txt",
			"/workspace/input.pdf",
		)
	}
	log.Debug().Msgf("dump pdf to txt with gs command: %s", strings.Join(cmd.Args, " "))
	log.Info().Msg("About to check the pdf text to confirm no errors")
	// Run the command
	err = cmd.Run()
	log.Trace().Msg("Here just after run")
	if err != nil {
		return nil, fmt.Errorf("Error running docker command: %v\n", err)
	}
	log.Trace().Msg("Here before readfile")
	data, err := os.ReadFile(filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"))
	if err != nil {
		return nil, fmt.Errorf("error reading pdf txt output %v", err)
	}
	log.Trace().Msg("Here before checking for strings")
	if strings.Contains(string(data), "Uncaught runtime errors") {
		return nil, fmt.Errorf("'Uncaught runtime errors' string detected in PDF contents.")
	}
	if strings.Contains(string(data), "Error loading data: Failed to fetch") {
		return nil, fmt.Errorf("'Error loading data: Failed to fetch' string detected in PDF contents.")
	}
	log.Trace().Msg("read in the text")

	outputFilePath := filepath.Join(outputDirFullpath, "pdf-txtwrite.txt")

	// Read the contents of the file into a []byte
	fileBytes, err := os.ReadFile(outputFilePath)
	if err != nil {
		return nil, fmt.Errorf("couldnt read the output file from the pdf text extract")
	}
	log.Trace().Msgf("read this from text file: %s", string(fileBytes))
	job.extractedText = string(fileBytes)
	resumeExtractionToLayoutRawJSONText, err := t.openAIResumeExtraction(job, outputDirFullpath)
	return &ResumeExtractResult{
		ResumeJSONRaw:  resumeExtractionToLayoutRawJSONText,
		ExpectedSchema: expectResponseSchema,
	}, nil
}

type extractAttempt struct {
	content                   string
	lengthRatioRelatedToInput float64
}

func (t *Tuner) openAIResumeExtraction(job *ResumeExtractionJob, outputDir string) (string, error) {
	expectResponseSchema, err := t.GetExpectedResponseJsonSchema(job.Layout)
	if err != nil {
		return "", err
	}

	mainExtractPrompt1 := "Inspect the following resume text and extract all relevant details pertaining to all fields of the included json schema. The response should pay careful attention to mapping input data to sensible output fields and formats while including as much information from the resume text data as possible. "
	extractPrompts := map[string]string{
		"chrono":     "Pay attention to the timeline of work history companies and the project work done at each of them. ",
		"functional": "Pay attention to the general concepts behind the work and projects that were done and be sure to come up with several functional area titles and multiple key contributions within each functional area. ",
	}
	mainExtractPrompt2 := "The goal is to extract as much information as possible and create a repository of resume data spanning the entire career. "
	perLayoutExtractPrompt, ok := extractPrompts[job.Layout]
	if !ok {
		return "", errors.New("could not get main extract prompt?!")
	}
	prompt_parts := []string{
		mainExtractPrompt1,
		perLayoutExtractPrompt,
		mainExtractPrompt2,
		"\n--- start resume text data ---\n",
		job.extractedText,
		"\n--- end resume text data ---\n",
	}
	prompt := strings.Join(prompt_parts, "")

	roboTries := 0
	maxRoboTries := 7
	targetLength := len(stripStringOfWhiteSpace(job.extractedText))
	var content string

	apiMessages := []map[string]interface{}{
		{
			"role":    "system",
			"content": "You are a helpful resume data extraction assistant. The goal is total information extraction. Array fields in the output should be used to their fullest capability.",
		},
		{
			"role":    "user",
			"content": prompt,
		},
	}
	data := map[string]interface{}{
		//"model": "gpt-4o",
		"model":    "gpt-4o-mini",
		"messages": apiMessages,
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "candidate_resume",
				"strict": true,
				"schema": expectResponseSchema,
			},
		},
		"temperature": 0.7,
		//"temperature": 1.0,
		"user": job.UserID,
	}
	var attemptsOutput []extractAttempt
	for {
		roboTries++
		// doooo

		api_request_pretty, err := serializeToJSON(data)
		if err != nil {
			return "", fmt.Errorf("Failed to marshal final JSON: %v", err)
		}
		err = writeToFile(api_request_pretty, roboTries, "api_request_pretty", outputDir)
		if err != nil {
			return "", fmt.Errorf("Failed to log api request locally: %v", err)
		}

		exists, output, err := checkForPreexistingAPIOutput(outputDir, "api_response_raw", roboTries)
		if err != nil {
			return "", fmt.Errorf("Error checking for pre-existing API output: %v", err)
		}
		if !exists {
			output, err = t.makeAPIRequest(data, roboTries, "api_response_raw", outputDir)
			if err != nil {
				log.Error().Msgf("openai request had error: %s", err.Error())
				return "", err
			}
		}

		//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
		var apiResponse APIResponse
		err = json.Unmarshal([]byte(output), &apiResponse)
		if err != nil {
			log.Error().Msgf("Error deserializing API response: %v", err)
			return "", err
		}

		//Extract the message content
		if len(apiResponse.Choices) == 0 {
			return "", errors.New("no choices found in the API response")
		}

		content = apiResponse.Choices[0].Message.Content
		//write the response here so that if there is an error with it we can see what it was ??

		err = validateJSON(content)
		if err != nil {
			log.Error().Msgf("Error validating JSON content: %v", err)
			return "", err
		}
		log.Info().Msgf("Got %d bytes of JSON content (at least well formed enough to be decodable) out of that last response", len(content))

		var results interface{}
		err = json.Unmarshal([]byte(content), &results)
		if err != nil {
			return "", err
		}
		resultContentDecodedExtractedWhitespaceStripped := stripStringOfWhiteSpace(ExtractText(results))
		extractedContentsStrippedLength := len(resultContentDecodedExtractedWhitespaceStripped)
		ratioExtractToInput := float64(extractedContentsStrippedLength) / float64(targetLength)

		log.Info().Msgf("input len %d (whitespace stripped text extract)", targetLength)
		log.Info().Msgf("output len %d (whitespace stripped text extract)", extractedContentsStrippedLength)
		log.Info().Msg(resultContentDecodedExtractedWhitespaceStripped)
		log.Info().Msgf("got output to input ratio of %0.2f", ratioExtractToInput)

		attemptsOutput = append(attemptsOutput, extractAttempt{
			content:                   content,
			lengthRatioRelatedToInput: ratioExtractToInput,
		})
		if roboTries >= maxRoboTries {
			log.Info().Msgf("tried enough now, stopping.")
			break
		}

		tryAgain := false
		tryAgainPrompt := ""
		if ratioExtractToInput < MIN_ACCEPTABLE_RATIO {
			tryAgain = true
			increaseByPct := int(((float64(targetLength) / float64(extractedContentsStrippedLength)) - 1) * 100)
			tryAgainPrompt = fmt.Sprintf("extraction was too short, please try again and increase the total output length by ~%d percent, retaining as much relevant information as possible. ", increaseByPct)
			log.Info().Msgf("we should ask it to try to get more by asking: '%s'", tryAgainPrompt)
		}
		if ratioExtractToInput > MAX_ACCEPTABLE_RATIO {
			tryAgain = true
			reduceByPct := int((1.0 - (1.0 / ratioExtractToInput)) * 100.0)
			tryAgainPrompt = fmt.Sprintf("extraction was too long, please try again and reduce the total output length by ~%d percent, while still retaining as much relevant information as possible. ", reduceByPct)
			log.Info().Msgf("we should ask it to try to get less by asking: '%s'", tryAgainPrompt)
		}

		if !tryAgain {
			log.Info().Msgf("good enough?")
			break
		}

		log.Info().Msgf("going to try again ...")
		data["messages"] = append(apiMessages, []map[string]interface{}{
			{
				"role":    "assistant",
				"content": content,
			}, {
				"role":    "user",
				"content": tryAgainPrompt,
			},
		}...)
	}

	//this is just a string atm dunno if thats good enough lol
	return getBestAttemptedExtract(attemptsOutput).content, nil
}

func getBestAttemptedExtract(attempts []extractAttempt) extractAttempt {
	if len(attempts) == 0 {
		return extractAttempt{} // handle empty input case
	}

	best := attempts[0]
	for _, attempt := range attempts {
		// Check if it's within the acceptable range first
		if isBetterAttempt(attempt, best) {
			best = attempt
		}
	}
	return best
}

// isBetterAttempt checks if `a` is a better attempt than `b`, based on proximity to 1.0
// and favors longer over shorter if equally close.
func isBetterAttempt(a, b extractAttempt) bool {
	// Calculate the absolute difference from 1.0 for both attempts
	diffA := math.Abs(a.lengthRatioRelatedToInput - 1.0)
	diffB := math.Abs(b.lengthRatioRelatedToInput - 1.0)

	// Prefer the one closer to 1.0
	if diffA < diffB {
		return true
	}

	// If they are equally close, prefer the one that is slightly longer
	if diffA == diffB && a.lengthRatioRelatedToInput > b.lengthRatioRelatedToInput {
		return true
	}

	return false
}

// GuessCandidateName extracts the candidate's full name from the resumeData interface
func (t *Tuner) GuessCandidateName(resumeData interface{}) (string, error) {
	// Type assert resumeData as a map
	dataMap, ok := resumeData.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("resumeData is not a map")
	}

	// Extract "personal_info" field and ensure it's a map
	personalInfo, ok := dataMap["personal_info"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("personal_info not found or is not a map")
	}

	// Extract the "name" field from personal_info
	name, ok := personalInfo["name"].(string)
	if !ok {
		return "", fmt.Errorf("name not found or is not a string")
	}

	// Return the extracted name
	return name, nil
}
