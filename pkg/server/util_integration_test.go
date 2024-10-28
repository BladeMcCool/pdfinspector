//go:build e2e

// ///////
// And remember to set a GOOGLE_APPLICATION_CREDENTIALS env var to something (json file creds) with that can access a bucket.
// ///////

package server

import (
	"context"
	"os"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/jobrunner"
	"pdfinspector/pkg/tuner"
	"testing"
)

func TestGCSObjectListing(t *testing.T) {
	serviceConfig := &config.ServiceConfig{
		GcsBucket: "my-stinky-bucket",
	}

	tuner := tuner.NewTuner(serviceConfig)
	server := &pdfInspectorServer{
		jobRunner: &jobrunner.JobRunner{
			Tuner: tuner,
		},
		config: serviceConfig,
	}
	fuck, _ := os.Getwd()
	t.Logf("fucking fuck %s", fuck)
	testUserID := "100009913768487635126" //my sso id, which has some generations at the moment (todo: isolate this test by having it set up and tear down the fixtures in GCS as part of the test)
	gensInfo, err := server.ListSSOUserGenerations(context.Background(), testUserID)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Log("obtained these generations info")
	for _, x := range gensInfo {
		t.Logf("%+v", x)
	}

}
