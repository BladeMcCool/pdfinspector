package config

import (
	"errors"
)

type JSONSchema struct {
	Schema               string                 `json:"$schema,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties bool                   `json:"additionalProperties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"` // For arrays
}

// Utility function to determine if a map contains a key
func hasKey(m map[string]interface{}, key string) bool {
	_, exists := m[key]
	return exists
}

// Recursive function to process a JSON schema and extract relevant information
//
//	iirc the reason i am doing this is so that the "title" and "default" schema fields don't go into the
//	request to openAI api, but i want to define them in the schema files that we will also use to load the react-json-schema-form
//	on the front-end.
func ExtractRelevantSchema(input interface{}) map[string]interface{} {
	// Attempt to type assert input as a map[string]interface{}
	schemaMap, ok := input.(map[string]interface{})
	if !ok {
		// If the input isn't a map (which it should be), return an empty map
		return map[string]interface{}{}
	}

	result := make(map[string]interface{})

	// Only extract top-level keys: $schema, type, properties, required, additionalProperties
	if hasKey(schemaMap, "$schema") {
		result["$schema"] = schemaMap["$schema"]
	}
	if hasKey(schemaMap, "type") {
		result["type"] = schemaMap["type"]
	}

	// Extract "required" if present
	if hasKey(schemaMap, "required") {
		result["required"] = schemaMap["required"]
	}

	// Extract "additionalProperties" if present
	if hasKey(schemaMap, "additionalProperties") {
		result["additionalProperties"] = schemaMap["additionalProperties"]
	}

	// Extract "additionalProperties" if present
	if hasKey(schemaMap, "description") {
		result["description"] = schemaMap["description"]
	}

	// Extract the "properties" key and recursively process each property
	if hasKey(schemaMap, "properties") {
		properties := schemaMap["properties"].(map[string]interface{})
		strippedProperties := make(map[string]interface{})
		for propName, propValue := range properties {
			//log.Trace().Msgf("propValue: %#v", propValue)
			propValueMap := propValue.(map[string]interface{})
			strippedProperties[propName] = ExtractRelevantSchema(propValueMap)
		}
		result["properties"] = strippedProperties
	}

	// Handle arrays (with "items")
	if hasKey(schemaMap, "items") {
		items := schemaMap["items"].(map[string]interface{})
		result["items"] = ExtractRelevantSchema(items)
	}

	return result
}

// EnhanceSchemaWithRendererFields is intended to add in a few extra fields that only really exist for the ReactResume project to have a bit more fine grained control over display
// some fields that are not really part of the current layout but that i want to maintain for potentially re-adding later in the fully personal resumedata that i have baked into the project.
func EnhanceSchemaWithRendererFields(layout string, input interface{}) (map[string]interface{}, error) {
	inputAsMap, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("input was not a map[string]interface{}")
	}
	properties, ok := inputAsMap["properties"].(map[string]interface{})
	if !ok {
		return nil, errors.New("'properties' was not a map[string]interface{}")
	}
	_ = properties
	properties["layout"] = map[string]interface{}{
		"type": "string",
	}

	switch layout {
	case "chrono":
		//chrono should tolerate these extra fields
		//might be better to define this as a schema file and find a way to merge them? idk i dont really want to maintain this but the use case is to allow me to hack on the resume render system while using the frontend to manage json tryouts.
		wh_props, ok := properties["work_history"].(map[string]interface{})["items"].(map[string]interface{})["properties"].(map[string]interface{})
		if !ok {
			return nil, errors.New("'work_history' (or subfield?) was not a map[string]interface{}")
		}
		wh_props["hide"] = map[string]interface{}{
			"type": "boolean",
		}
		wh_props["companydesc"] = map[string]interface{}{
			"type": "string",
		}
		wh_props["sortdate"] = map[string]interface{}{
			"type": "string",
		}

		projects_props, ok := wh_props["projects"].(map[string]interface{})["items"].(map[string]interface{})["properties"].(map[string]interface{})
		if !ok {
			return nil, errors.New("'projects' (or subfield?) was not a map[string]interface{}")
		}
		projects_props["hide"] = map[string]interface{}{
			"type": "boolean",
		}
		projects_props["dates"] = map[string]interface{}{
			"type": "string",
		}
		projects_props["sortdate"] = map[string]interface{}{
			"type": "string",
		}
		projects_props["tech"] = map[string]interface{}{
			"type": "string", //should probably turn this into a list like the other layout uses for 'tech'
		}

		projects_props["printOff"] = map[string]interface{}{
			"type": "boolean", //not sure if i should remove this entirely
		}
		projects_props["pageBreakBefore"] = map[string]interface{}{
			"type": "boolean", //not sure if i should remove this entirely
		}

	case "functional":
		//functional layout should tolerate these extra fields
		fa_props, ok := properties["functional_areas"].(map[string]interface{})["items"].(map[string]interface{})["properties"].(map[string]interface{})
		if !ok {
			return nil, errors.New("'functional_areas' (or subfield?) was not a map[string]interface{}")
		}

		kc_props, ok := fa_props["key_contributions"].(map[string]interface{})["items"].(map[string]interface{})["properties"].(map[string]interface{})
		if !ok {
			return nil, errors.New("'projects' (or subfield?) was not a map[string]interface{}")
		}
		kc_props["hide"] = map[string]interface{}{
			"type": "boolean",
		}

	}
	return inputAsMap, nil
}
