package server

//todo decide that we aren't going down this road at all... and to just delete this ....
//////
//router.Post("/authcodetest", s.authcodetest)
//router.Post("/tokentest", s.tokentest)
//////

//
//// Request body for exchanging code or token
//type AuthRequest struct {
//	Code  string `json:"code,omitempty"`
//	Token string `json:"access_token,omitempty"`
//}
//
//// Response for sending user data back to the client
//type AuthResponse struct {
//	Sub string `json:"sub"`
//}

//// GetAPIToken handles the verification of the Google ID token sent from the client.
//func (s *pdfInspectorServer) authcodetest(w http.ResponseWriter, r *http.Request) {
//	log.Info().Msg("here in authcodetest")
//
//	googleConfig := &oauth2.Config{
//		ClientID:     s.config.FrontendClientID,
//		ClientSecret: s.config.FrontendClientSecret,
//		RedirectURL:  "http://localhost:3003",
//		Scopes:       []string{"openid", "profile", "email"},
//		Endpoint:     google.Endpoint,
//	}
//	log.Info().Msgf("here in authcodetest, the googleConfig looks like: %#v", googleConfig)
//
//	var authReq AuthRequest
//	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
//		http.Error(w, "invalid request", http.StatusBadRequest)
//		return
//	}
//	log.Info().Msgf("here in authcodetest, the authReq looks like: %#v", authReq)
//
//	// Exchange authorization code for tokens
//	token, err := googleConfig.Exchange(context.Background(), authReq.Code)
//	if err != nil {
//		http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusInternalServerError)
//		return
//	}
//	log.Info().Msgf("got this token result: %#v", token)
//
//	// Verify and parse the ID token to extract user info
//	idToken := token.Extra("id_token").(string)
//	userInfo, err := verifyIDToken(s.config.FrontendClientID, idToken)
//	if err != nil {
//		http.Error(w, fmt.Sprintf("ID token verification failed: %v", err), http.StatusInternalServerError)
//		return
//	}
//	log.Info().Msgf("authcodetest got this: %#v", userInfo)
//	//json.NewEncoder(w).Encode(AuthResponse{Sub: userInfo.Sub})
//}
//
//func (s *pdfInspectorServer) tokentest(w http.ResponseWriter, r *http.Request) {
//	log.Info().Msg("here in tokentest")
//
//	var authReq AuthRequest
//	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
//		http.Error(w, "invalid request", http.StatusBadRequest)
//		return
//	}
//
//	log.Info().Msgf("here in tokentest, the authReq looks like: %#v", authReq)
//
//	// Verify the token sent from the client
//	userInfo, err := verifyIDToken(s.config.FrontendClientID, authReq.Token)
//	if err != nil {
//		http.Error(w, fmt.Sprintf("token verification failed: %v", err), http.StatusInternalServerError)
//		return
//	}
//
//	log.Info().Msgf("tokentest got this: %#v", userInfo)
//	//json.NewEncoder(w).Encode(AuthResponse{Sub: userInfo.Sub})
//}
//
//// Helper function to verify the ID token and extract user information
//func verifyIDToken(clientID, idToken string) (*idtoken.Payload, error) {
//	// Verify the ID token with Google's verification API
//	payload, err := idtoken.Validate(context.Background(), idToken, clientID)
//	if err != nil {
//		return nil, err
//	}
//
//	return payload, nil
//}
//
////////////////////////////////////////////////
