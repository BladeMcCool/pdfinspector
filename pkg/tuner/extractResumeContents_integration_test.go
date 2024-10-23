//go:build e2e

// ///////
// And remember to set a OPENAI_API_KEY env var to something with api credit or stuff won't work.
// ///////
package tuner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	"testing"
)

func loadFixtureContents(name string) []byte {
	wd, _ := os.Getwd()
	file, err := os.Open(filepath.Join(wd, "..", "..", "test_fixtures", name))
	if err != nil {
		panic(err)
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}
	return contents
}
func getTestOutputDir(skipEnsure bool) string {
	wd, _ := os.Getwd()
	outputDir := filepath.Join(wd, "..", "..", "test_output")
	if !skipEnsure {
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			panic(err)
		}
	}
	return outputDir
}
func getSchemasDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "response_templates")
}
func getTestAPIKey() string {
	//well, actually a real api key b/c we are going to use the actual api here.
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("no api key, won't be able to run tests in this file.")
	}
	return apiKey
}

var testOutputDir = getTestOutputDir(false)
var schemasDir = getSchemasDir()
var testApiKey = getTestAPIKey()

func cleanPriorTestOutput() {
	//remove all existing files in the path named by testOutputDir ...
	files, err := os.ReadDir(testOutputDir)
	if err != nil {
		panic(err)
	}
	for _, v := range files {
		toRemove := filepath.Join(testOutputDir, v.Name())
		fmt.Printf("would remove: %s", toRemove)
		err = os.RemoveAll(toRemove)
		if err != nil {
			panic(err)
		}
	}
}

func TestOpenAIResumeExtractProducesRoughlyTheCorrectLengthImportedJSON(t *testing.T) {
	t.Log("should only see this for e2e enabled")
	//grab the contents of the fixture

	cleanPriorTestOutput()
	fixture := string(loadFixtureContents("longchrono-pdf-to-text.txt"))
	//t.Log(string(stripStringOfWhiteSpace(fixture)))
	//check the number of characters of content
	fixtureStripped := stripStringOfWhiteSpace(fixture)
	charLen := len(fixtureStripped)
	t.Logf("Loaded %d chars of space-stripped-input", charLen)
	//engage the 'ai extraction' process
	testTuner := &Tuner{
		config: &config.ServiceConfig{
			SchemasPath:  schemasDir,
			OpenAiApiKey: testApiKey,
		},
		Fs: nil,
	}

	resultContentJSON, err := testTuner.openAIResumeExtraction(&ResumeExtractionJob{
		FileContent:   nil,
		extractedText: fixture,
		Layout:        "functional",
		UseSystemGs:   false,
		UserID:        "test-user",
	}, testOutputDir)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Logf("resultContent JSON length here in the test before we strip it down to check properly: %d", len(resultContentJSON))

	//check the length of contents of the result
	var results interface{}
	err = json.Unmarshal([]byte(resultContentJSON), &results)
	if err != nil {
		t.Fatal(err.Error())
	}
	//ex1 := ExtractText(results)
	//assert.Equal(t, ex1, ex2)
	resultContentExtractedForLengthCheck := ExtractText(results)
	t.Logf("resultContentExtractedForLengthCheck length before stripping whitespace %d", len(resultContentExtractedForLengthCheck))
	extractedStripped := stripStringOfWhiteSpace(resultContentExtractedForLengthCheck)
	t.Logf("extractedStripped length (whitespace removed) %d", len(extractedStripped))

	resultContentLength := len(extractedStripped)
	ratioExtractToInput := float64(resultContentLength) / float64(charLen)
	t.Logf("input  stripped text: %s", fixtureStripped)
	t.Logf("output stripped text: %s", extractedStripped)
	t.Logf("length input  stripped text: %d", charLen)
	t.Logf("length output stripped text: %d", resultContentLength)
	t.Logf("noted a ratioExtractToInput of %0.2f", ratioExtractToInput)
	if ratioExtractToInput < MIN_ACCEPTABLE_RATIO {
		t.Fatalf("still kinda too short.")
	}
	if ratioExtractToInput > MAX_ACCEPTABLE_RATIO {
		t.Fatalf("still kinda too long.")
	}
	t.Logf("we will call a ratio of %0.2f a pass.", ratioExtractToInput)
}
