package tuner

import (
	"encoding/json"
	"testing"
)

// TestGuessCandidateName tests the GuessCandidateName function using sample data
func TestGuessCandidateName(t *testing.T) {
	// Sample test case 1
	rawData1 := `
	{
		"education_v2": [
			{
				"notes": ["Notes about education"],
				"institution": "Institution Name",
				"location": "Institution Location",
				"description": "Degree/Certificate Description",
				"graduated": "Graduation Date"
			}
		],
		"personal_info": {
			"name": "Dr Potato",
			"email": "email@example.com",
			"phone": "123-456-7890",
			"linkedin": "https://linkedin.com/in/placeholder",
			"location": "Location Placeholder",
			"github": "https://github.com/placeholder"
		},
		"skills": ["Example Skill 1", "Example Skill 2"],
		"work_history": [
			{
				"projects": [
					{
						"desc": "Project Description",
						"github": "https://github.com/placeholder",
						"location": "Project Location",
						"jobtitle": "Project Job Title"
					}
				],
				"company": "Company Name",
				"tag": "company-tag",
				"location": "Company Location",
				"jobtitle": "Job Title",
				"daterange": "Date Range"
			}
		]
	}`

	// Sample test case 2
	rawData2 := `
	{
		"personal_info": {
			"name": "Ronson McJonson",
			"email": "email@example.com",
			"phone": "123-456-7890",
			"linkedin": "https://linkedin.com/in/placeholder",
			"location": "Location Placeholder",
			"github": "https://github.com/placeholder"
		}
	}`

	// Define test cases
	tests := []struct {
		rawData      string
		expectedName string
	}{
		{rawData1, "Dr Potato"},
		{rawData2, "Ronson McJonson"},
	}

	tuner := &Tuner{
		config: nil,
		Fs:     nil,
	}

	// Loop over test cases
	for _, tc := range tests {
		// Unmarshal JSON data
		var resumeData interface{}
		err := json.Unmarshal([]byte(tc.rawData), &resumeData)
		if err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Call GuessCandidateName

		name, err := tuner.GuessCandidateName(resumeData)
		if err != nil {
			t.Fatalf("GuessCandidateName failed: %v", err)
		}

		// Check if the result matches the expected name
		if name != tc.expectedName {
			t.Errorf("Expected name %q, but got %q", tc.expectedName, name)
		}
	}
}
