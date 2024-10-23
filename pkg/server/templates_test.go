package server

import (
	"encoding/json"
	"errors"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/tuner"
	"testing"
)

var invalidResumeDataJSON = `{
	"name": "John Doe",
	"email": "johndoe@example.com",
	"skills": ["Go", "JavaScript"]
}`

var validChronoResumeDataJSON = `{
  "education_v2": [
    {
      "description": "BS in Early Childhood Development",
      "graduated": "1999",
      "institution": "University of Arkansas at Little Rock",
      "location": "Little Rock, AR",
      "notes": [
        "GPA (4.0 Scale): Early Childhood Development – 3.8"
      ]
    },
    {
      "description": "BA in Elementary Education",
      "graduated": "1998",
      "institution": "University of Arkansas at Little Rock",
      "location": "Little Rock, AR",
      "notes": [
        "GPA (4.0 Scale): Elementary Education – 3.5"
      ]
    }
  ],
  "personal_info": {
    "email": "jwsmith@colostate.edu",
    "github": null,
    "linkedin": null,
    "location": "Fort Collins, CO",
    "name": "John W. Smith",
    "phone": ""
  },
  "skills": [
    "Early Childhood Development",
    "Special Needs Care",
    "Volunteer Coordination",
    "Client Database Management",
    "Activity Planning",
    "Financial Assistance Research"
  ],
  "work_history": [
    {
      "company": "The Wesley Center",
      "daterange": "1999-2002",
      "jobtitle": "Counseling Supervisor",
      "location": "Little Rock, Arkansas",
      "projects": [],
      "tag": ""
    },
    {
      "company": "Rainbow Special Care Center",
      "daterange": "1997-1999",
      "jobtitle": "Client Specialist",
      "location": "Little Rock, Arkansas",
      "projects": [],
      "tag": ""
    },
    {
      "company": "Cowell Elementary",
      "daterange": "1996-1997",
      "jobtitle": "Teacher’s Assistant",
      "location": "Conway, Arkansas",
      "projects": [],
      "tag": ""
    }
  ]
}
`

var validChronoResumeDataWithRendererFieldsJSON = `{
    "layout": "chrono",
    "education_v2": [
        {
            "description": "BS in Early Childhood Development",
            "graduated": "1999",
            "institution": "University of Arkansas at Little Rock",
            "location": "Little Rock, AR",
            "notes": [
                "GPA (4.0 Scale): Early Childhood Development – 3.8"
            ]
        },
        {
            "description": "BA in Elementary Education",
            "graduated": "1998",
            "institution": "University of Arkansas at Little Rock",
            "location": "Little Rock, AR",
            "notes": [
                "GPA (4.0 Scale): Elementary Education – 3.5"
            ]
        }
    ],
    "personal_info": {
        "email": "jwsmith@colostate.edu",
        "github": null,
        "linkedin": null,
        "location": "Fort Collins, CO",
        "name": "John W. Smith",
        "phone": ""
    },
    "skills": [
        "Early Childhood Development",
        "Special Needs Care",
        "Volunteer Coordination",
        "Client Database Management",
        "Activity Planning",
        "Financial Assistance Research"
    ],
    "work_history": [
        {
            "company": "The Wesley Center",
            "daterange": "1999-2002",
            "jobtitle": "Counseling Supervisor",
            "location": "Little Rock, Arkansas",
            "hide": true,
            "projects": [
                {
                    "desc": "Did a thing",
                    "dates": "2011",
                    "tech": "CFM, PHP, MySQL",
                    "location": "Remote",
					"github":"",
                    "hide": false
                },
                {
                    "desc": "Built stuff",
                    "dates": "1999",
                    "tech": "VBScript, HTML",
                    "location": "Remote",
					"github":"this can be empty but not missing",
                    "hide": true
                }
            ],
            "tag": ""
        },
        {
            "company": "Rainbow Special Care Center",
            "daterange": "1997-1999",
            "jobtitle": "Client Specialist",
            "location": "Little Rock, Arkansas",
            "projects": [],
            "tag": ""
        },
        {
            "company": "Cowell Elementary",
            "daterange": "1996-1997",
            "jobtitle": "Teacher’s Assistant",
            "location": "Conway, Arkansas",
            "projects": [],
            "tag": ""
        }
    ]
}`

var validFunctionalResumeDataWithRendererFieldsJSON = `{
    "layout": "functional",
    "education": [
        {
            "description": "Education Description",
            "graduated": "Graduation Date",
            "institution": "Institution Name",
            "location": "Institution Location",
            "notes": [
                "Notes about education"
            ]
        }
    ],
    "employment_history": [
        {
            "company": "Company Name",
            "daterange": "Date Range",
            "location": "Company Location",
            "title": "Job Title"
        }
    ],
    "functional_areas": [
        {
            "key_contributions": [
                {
                    "company": "Company Name",
                    "daterange": "Date Range",
                    "description": "Description of key contributions",
                    "lead_in": 0,
                    "hide":false,
                    "tech": [
                        "Technology/Skills"
                    ]
                }
            ],
            "title": "Functional Area Title"
        }
    ],
    "overview": "Overview or Summary Text",
    "personal_info": {
        "email": "email@example.com",
        "github": "https://github.com/placeholder",
        "linkedin": "https://linkedin.com/in/placeholder",
        "location": "Location Placeholder",
        "name": "Full Name",
        "phone": "123-456-7890"
    }
}`

func getResponseTemplatesDir() string {
	testsDir, err := os.Getwd()
	if err != nil {
		panic("wat")
	}
	return filepath.Join(testsDir, "../../response_templates")
}

var testServer *pdfInspectorServer

func TestMain(m *testing.M) {
	// Set up the server once for all tests
	testServer = NewPdfInspectorServer(&config.ServiceConfig{SchemasPath: getResponseTemplatesDir()})
	// Run all tests
	m.Run()
}

func TestValidateInvalidResumeDataAgainstTemplateSchema(t *testing.T) {
	// Instantiate pdfInspectorServer with the mock jobRunner
	//server := NewPdfInspectorServer(&config.ServiceConfig{SchemasPath: getResponseTemplatesDir()})
	// Hardcoded resume data as a string (placeholder content)

	// Deserialize the hardcoded resume data into an interface
	var resumeData interface{}
	err := json.Unmarshal([]byte(invalidResumeDataJSON), &resumeData)
	assert.NoError(t, err, "Failed to unmarshal resume data")

	// Call the function to validate the resume data against the schema
	err = testServer.validateResumeDataAgainstTemplateSchema("functional", resumeData, false)
	err_spew := spew.Sprint(err)
	t.Logf("error details: %#v", spew.Sprint(err))
	assert.True(t, errors.Is(err, &tuner.SchemaValidationError{}))
	assert.Contains(t, err_spew, "personal_info is required", "Error message should contain 'personal_info is required'")
	// Assert that there was no validation error
	assert.Error(t, err, "Validation failed to produce an error for invalid input.")
}

func TestValidateResumeDataWithoutRendererFieldsAgainstResponseTemplateSchema(t *testing.T) {
	// Deserialize the hardcoded resume data into an interface
	var resumeData interface{}
	err := json.Unmarshal([]byte(validChronoResumeDataJSON), &resumeData)
	assert.NoError(t, err, "Failed to unmarshal resume data")

	// Call the function to validate the resume data against the schema
	err = testServer.validateResumeDataAgainstTemplateSchema("chrono", resumeData, false)
	assert.Nil(t, err)
}

func TestValidateChronoResumeDataWithRendererFieldsAgainstRendererTemplateSchema(t *testing.T) {
	// Deserialize the hardcoded resume data into an interface
	var resumeData interface{}
	err := json.Unmarshal([]byte(validChronoResumeDataWithRendererFieldsJSON), &resumeData)
	assert.NoError(t, err, "Failed to unmarshal resume data")

	// Call the function to validate the resume data against the schema
	err = testServer.validateResumeDataAgainstTemplateSchema("chrono", resumeData, true)
	if err != nil {
		t.Logf("err: %s", err.Error())
	}
	assert.Nil(t, err)
}

func TestValidateFunctionalResumeDataWithRendererFieldsAgainstRendererTemplateSchema(t *testing.T) {
	// Deserialize the hardcoded resume data into an interface
	var resumeData interface{}
	err := json.Unmarshal([]byte(validFunctionalResumeDataWithRendererFieldsJSON), &resumeData)
	assert.NoError(t, err, "Failed to unmarshal resume data")

	// Call the function to validate the resume data against the schema
	err = testServer.validateResumeDataAgainstTemplateSchema("functional", resumeData, true)
	if err != nil {
		t.Logf("err: %s", err.Error())
	}
	assert.Nil(t, err)
}
