package server

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"net/http"
	"sort"
	"strings"
	"time"
)

type generationInfo struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
}

// Custom JSON struct to format the date as yyyy-mm-dd hh:mm
type generationInfoFormatted struct {
	Name    string `json:"name"`
	Created string `json:"created"`
}

type attemptInfo struct {
	Name    string
	Created time.Time
	Data    string
}

// CreateCustomToken creates a JWT with 'sub' and 'apikey'
func (s *pdfInspectorServer) CreateCustomToken(sub string) (string, error) {
	claims := jwt.MapClaims{
		"sub": sub,                                         // User identifier
		"exp": time.Now().Add(365 * 24 * time.Hour).Unix(), // Expiration time (1 year)
	}

	// Create the JWT with the claims and sign it with the secret key
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.config.JwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return signedToken, nil
}

// ValidateCustomToken verifies the JWT and returns the claims if valid
func (s *pdfInspectorServer) ValidateCustomToken(tokenString string) (jwt.MapClaims, error) {
	// Parse and validate the JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Return the secret key for verifying the token signature
		return []byte(s.config.JwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name})) // Specify allowed algorithms here

	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Extract and return the claims if the token is valid
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token - no claims?")
	}

	return claims, nil
}

func (s *pdfInspectorServer) extractBearerToken(r *http.Request) (string, error) {
	// Extract the Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("No authorization header provided")
	}

	// Expect a Bearer token
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
		return "", errors.New("Malformed authorization header")
	}

	return tokenParts[1], nil
}

// ListSSOUserGenerations lists all objects under the given prefix in a GCS bucket.
func (s *pdfInspectorServer) ListSSOUserGenerations(ctx context.Context, userId string) ([]generationInfo, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	prefix := fmt.Sprintf("sso/%s/gen/", userId)

	// Query to list objects with the specified prefix
	query := &storage.Query{Prefix: prefix}
	it := bucket.Objects(ctx, query)

	var genIds []generationInfo
	for {
		objAttr, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error retrieving object: %w", err)
		}

		// Append the object name to the slice
		// Extract the genId by stripping the prefix
		genId := strings.TrimPrefix(objAttr.Name, prefix)
		genIds = append(genIds, generationInfo{
			Name:    genId,
			Created: objAttr.Created,
		})
	}
	return genIds, nil
}

// ListSSOUserGenerations lists all objects under the given prefix in a GCS bucket.
func (s *pdfInspectorServer) GetOutputGenerationsJSON(ctx context.Context, genId string) ([]attemptInfo, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	prefix := fmt.Sprintf("outputs/%s/attempt", genId)
	log.Trace().Msgf("GetOutputGenerationsJSON for prefix %s", prefix)
	// Query to list objects with the specified prefix
	query := &storage.Query{Prefix: prefix}
	it := bucket.Objects(ctx, query)

	var attempts []attemptInfo
	for {
		objAttr, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error retrieving object attrs: %w", err)
		}

		// Append the object name to the slice
		// Extract the genId by stripping the prefix
		attempt := strings.TrimPrefix(objAttr.Name, prefix)
		contents, err := s.readTemplateFromGCS(ctx, objAttr.Name)
		log.Info().Msgf("read some contents: %s", contents)
		if err != nil {
			if err != nil {
				return nil, fmt.Errorf("error retrieving object: %w", err)
			}
		}
		attempts = append(attempts, attemptInfo{
			Name:    attempt,
			Created: objAttr.Created,
			Data:    string(contents),
		})
	}
	return attempts, nil
}

// Sort by date descending and serialize to JSON with custom date format
func sortAndSerializeGenerations(generations []generationInfo) (string, error) {
	// Sort the slice by Created in descending order
	sort.Slice(generations, func(i, j int) bool {
		return generations[i].Created.After(generations[j].Created)
	})

	//// Create a new slice for the formatted results
	//var formattedGenerations []generationInfoFormatted
	//
	//// Format the Created time and copy the data to the formatted slice
	//for _, gen := range generations {
	//	formattedGenerations = append(formattedGenerations, generationInfoFormatted{
	//		Name:    gen.Name,
	//		Created: gen.Created.Format("2006-01-02 15:04"), // Custom date format yyyy-mm-dd hh:mm
	//	})
	//}
	//
	//// Serialize to JSON
	//jsonData, err := json.Marshal(formattedGenerations)

	jsonData, err := json.Marshal(generations)

	if err != nil {
		return "", fmt.Errorf("failed to serialize to JSON: %w", err)
	}

	return string(jsonData), nil
}
