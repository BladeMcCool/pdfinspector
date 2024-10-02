package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The extractRelevantSchema function is assumed to be defined as previously described.
// Add the full function definition here if needed for completeness.

func TestExtractRelevantSchema(t *testing.T) {
	// Input schema with irrelevant fields (like "title", "description", etc.)
	rawSchema := `
	{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "title": "Person Information",
	  "description": "This schema defines a person’s information",
	  "type": "object",
	  "properties": {
	    "name": {
	      "type": "string",
	      "title": "Full Name"
	    },
	    "age": {
	      "type": "integer",
	      "description": "Age in years"
	    },
	    "address": {
	      "type": "object",
	      "title": "Address",
	      "properties": {
	        "street": {
	          "type": "string",
	          "title": "Street Name"
	        },
	        "city": {
	          "type": "string",
	          "description": "The city name"
	        }
	      },
	      "required": ["street"]
	    },
	    "tags": {
	      "type": "array",
	      "items": {
	        "type": "string"
	      },
	      "description": "Tags for the person"
	    }
	  },
	  "required": ["name", "age"],
	  "additionalProperties": false
	}`

	// Expected stripped-down schema (with only the relevant fields)
	expectedSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "integer",
			},
			"address": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"street": map[string]interface{}{
						"type": "string",
					},
					"city": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"street"},
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required":             []interface{}{"name", "age"},
		"additionalProperties": false,
	}

	// Parse the raw schema into a map[string]interface{}
	var inputSchema map[string]interface{}
	err := json.Unmarshal([]byte(rawSchema), &inputSchema)
	if err != nil {
		t.Fatalf("Error unmarshalling input schema: %v", err)
	}

	// Run the function to extract the relevant schema
	strippedSchema := ExtractRelevantSchema(inputSchema)

	// Compare the stripped schema with the expected output
	if !reflect.DeepEqual(strippedSchema, expectedSchema) {
		// Output the difference for debugging
		strippedJSON, _ := json.MarshalIndent(strippedSchema, "", "  ")
		expectedJSON, _ := json.MarshalIndent(expectedSchema, "", "  ")
		t.Errorf("Stripped schema does not match expected schema.\nGot:\n%s\n\nExpected:\n%s", strippedJSON, expectedJSON)
	}
}

func TestExtractLayoutSchemasWorks(t *testing.T) {

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}

	// Build the absolute path to the response_templates directory, going up two levels
	filePath := filepath.Join(cwd, "..", "..", "response_templates")

	// Define the test case struct inline
	tests := []struct {
		filenameFragment string // The file to process in the test
		layout           string // A label to show alongside the test case
	}{
		{filenameFragment: "chrono", layout: "Chrono"},
		//{filenameFragment: "functional", layout: "Functional"},
	}

	// Iterate over each test case
	for _, tc := range tests {
		t.Run(fmt.Sprintf("Processing %s with %s", tc.filenameFragment, tc.layout), func(t *testing.T) {

			expectResponseFilePath := filepath.Join(filePath, fmt.Sprintf("%s-schema.json", tc.filenameFragment))
			// Read expect_response.json
			expectResponseContent, err := os.ReadFile(expectResponseFilePath)
			if err != nil {
				t.Errorf("error reading file from filesystem: %v", err)
				return
			}
			var result interface{}
			if err = json.Unmarshal(expectResponseContent, &result); err != nil {
				t.Errorf("error decoding JSON into interface{}: %v", err)
				return
			}

			ExtractRelevantSchema(result) //if it doesn't panic that's great :)
		})
	}
}
