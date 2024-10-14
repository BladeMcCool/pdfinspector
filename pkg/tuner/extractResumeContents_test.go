package tuner

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
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

func TestGetBestAttemptExtractPicksBest(t *testing.T) {
	attempts := []extractAttempt{{
		lengthRatioRelatedToInput: 0.8,
	}, {
		lengthRatioRelatedToInput: 0.9,
	}, {
		lengthRatioRelatedToInput: 1.0,
	}, {
		lengthRatioRelatedToInput: 1.1,
	}, {
		lengthRatioRelatedToInput: 1.6,
	}}
	best := getBestAttemptedExtract(attempts)
	assert.Equal(t, 1.0, best.lengthRatioRelatedToInput)
}

func TestGetBestAttemptExtractPicksLeastBadFromAllLong(t *testing.T) {
	attempts := []extractAttempt{{
		lengthRatioRelatedToInput: 0.2,
	}, {
		lengthRatioRelatedToInput: 1.3,
	}, {
		lengthRatioRelatedToInput: 1.4,
	}, {
		lengthRatioRelatedToInput: 5.5,
	}}
	best := getBestAttemptedExtract(attempts)
	assert.Equal(t, 1.3, best.lengthRatioRelatedToInput)
}

func TestGetBestAttemptExtractPicksLeastBadFromAllShort(t *testing.T) {
	attempts := []extractAttempt{{
		lengthRatioRelatedToInput: 0.2,
	}, {
		lengthRatioRelatedToInput: 0.3,
	}, {
		lengthRatioRelatedToInput: 0.4,
	}}
	best := getBestAttemptedExtract(attempts)
	assert.Equal(t, 0.4, best.lengthRatioRelatedToInput)
}

func TestGetBestAttemptExtractPicksLeastBadFromCloseToTarget(t *testing.T) {
	attempts := []extractAttempt{{
		lengthRatioRelatedToInput: 0.7,
	}, {
		lengthRatioRelatedToInput: 0.8,
	}, {
		lengthRatioRelatedToInput: 1.5,
	}, {
		lengthRatioRelatedToInput: 0.3,
	}, {
		lengthRatioRelatedToInput: 1.4,
	}}
	best := getBestAttemptedExtract(attempts)
	assert.Equal(t, 0.8, best.lengthRatioRelatedToInput)
}
