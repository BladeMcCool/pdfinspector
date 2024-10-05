package server

import (
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/idtoken"
	"net/http"
	"pdfinspector/pkg/filesystem"
)

// AuthMiddleware is the middleware that checks for a valid Bearer token and session information in GCS.
func (s *pdfInspectorServer) SSOUserDetectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.FrontendClientID == "" {
			//because if this is blank and then we just try to validate against a blank audience then it will accept any token i think regardless of audience or where it came from.
			log.Warn().Msgf("server was not configured with config.FrontendClientID, so we can't set the audience to restrict token validation.")
			next.ServeHTTP(w, r)
			return
		}

		credential := r.Header.Get("X-Credential")
		if credential == "" {
			//just means it wasnt provided. its only really needed for getting apikey anyway.
			next.ServeHTTP(w, r)
			return
		}

		var ssoSubject string
		gotSsoSubject := false
		bearerToken, _ := s.extractBearerToken(r)
		var useBearerToken *string
		if bearerToken != "" {
			useBearerToken = &bearerToken
		}
		mapClaims, err := s.ValidateCustomToken(credential, useBearerToken)
		//log.Info().Msgf("SSOUserDetectionMiddleware with credential %s and bearer %s", credential, bearerToken)
		log.Info().Msgf("SSOUserDetectionMiddleware with credential %s", credential)
		if err == nil {
			apikeyOwnership, ok := mapClaims["sub"].(string)
			if !ok {
				log.Error().Msg("no sub ?")
				http.Error(w, "Invalid ID token", http.StatusUnauthorized)
				return
			}
			log.Info().Msgf("set ssoSubject %s from custom token", apikeyOwnership)
			ssoSubject = apikeyOwnership
			gotSsoSubject = true
		} else {
			log.Info().Msgf("err from validatecustom: %v", err)
		}

		if !gotSsoSubject {
			//might be a google one during a login
			payload, err := idtoken.Validate(r.Context(), credential, s.config.FrontendClientID)
			if err != nil {
				http.Error(w, "Invalid ID token", http.StatusUnauthorized)
				return
			}

			// Step 3: Extract user identifier (e.g., email, sub) from the payload
			ssoSubject = payload.Subject // The unique user ID (sub claim)
			email, ok := payload.Claims["email"].(string)
			//log.Info().Msgf("we thinks %s just logged in, %#v", ssoSubject, payload)
			log.Info().Msgf("request from SSO Subject ID %s", ssoSubject)
			if !ok || email == "" {
				http.Error(w, "Email not found in token", http.StatusUnauthorized)
				return
			}
			gotSsoSubject = true
		}

		if !gotSsoSubject {
			//thats ok. move ahead without it.
			next.ServeHTTP(w, r)
			return
		}

		// If everything is valid, proceed to the next handler
		ctx := context.WithValue(r.Context(), "ssoSubject", ssoSubject)

		// Create a new request with the updated context
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware is the middleware that checks for a valid Bearer token and session information in GCS.
func (s *pdfInspectorServer) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken, err := s.extractBearerToken(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unauthorized: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		// Check if the bearer token matches the AdminKey in the config
		if bearerToken == s.config.AdminKey {
			// Allow the request through if it's an admin token
			// Set the admin flag in the context
			ctx := context.WithValue(r.Context(), "isAdmin", true)

			// Create a new request with the updated context
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
			return
		}

		knownApiKey, err := s.checkApiKeyExists(r.Context(), bearerToken)
		if err != nil || knownApiKey == false {
			// If the token is not a known user, deny access
			http.Error(w, "Unauthorized: Unknown user token", http.StatusUnauthorized)
			return
		}

		// If everything is valid, proceed to the next handler
		ctx := context.WithValue(r.Context(), "userKey", bearerToken)

		// Create a new request with the updated context
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// Method to check if an API key exists using GCS and cache results.
func (s *pdfInspectorServer) checkApiKeyExists(ctx context.Context, apiKey string) (bool, error) {
	// 1. First, check the map (knownUsers) for the API key.
	s.userKeysMu.RLock()
	isRealApiKey, hasRecordOfChecking := s.userKeys[apiKey]
	s.userKeysMu.RUnlock()

	// If the API key is found in the cache, return the cached result.
	if hasRecordOfChecking {
		return isRealApiKey, nil
	}

	// 2. If the API key is not found in the cache, check GCS.
	// First, check if s.jobRunner.Tuner.Fs is a GcpFilesystem.
	gcpFs, ok := s.jobRunner.Tuner.Fs.(*filesystem.GCSFileSystem)
	if !ok {
		return false, fmt.Errorf("file system is not GCSFileSystem")
	}

	// Use the GCS client to check for the file's existence.
	client := gcpFs.Client
	bucketName := s.config.GcsBucket
	objectName := fmt.Sprintf("users/%s/credit", apiKey) // Assuming the API keys are stored in a "keys" folder

	// 3. Check if the API key file exists in GCS.
	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	_, err := obj.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		// The API key file does not exist, cache the result as false.
		log.Info().Msgf("Discovered and cached the fact that api key %s does _not_ exist.", apiKey)
		s.userKeysMu.Lock()
		s.userKeys[apiKey] = false
		s.userKeysMu.Unlock()
		return false, nil
	} else if err != nil {
		// Some other error occurred.
		return false, fmt.Errorf("error checking API key in GCS: %v", err)
	}
	log.Info().Msgf("Discovered and cached the fact that api key %s exists.", apiKey)
	// 4. If the file exists, cache the result as true.
	s.userKeysMu.Lock()
	s.userKeys[apiKey] = true
	s.userKeysMu.Unlock()
	return true, nil
}
