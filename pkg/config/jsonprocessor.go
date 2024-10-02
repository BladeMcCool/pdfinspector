package config

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

	// Extract "required" if present
	if hasKey(schemaMap, "required") {
		result["required"] = schemaMap["required"]
	}

	// Extract "additionalProperties" if present
	if hasKey(schemaMap, "additionalProperties") {
		result["additionalProperties"] = schemaMap["additionalProperties"]
	}

	// Handle arrays (with "items")
	if hasKey(schemaMap, "items") {
		items := schemaMap["items"].(map[string]interface{})
		result["items"] = ExtractRelevantSchema(items)
	}

	return result
}
