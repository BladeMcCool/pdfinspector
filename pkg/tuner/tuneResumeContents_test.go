package tuner

import "testing"

func TestCorrectlyPicksBetterPNGInspectResultFromMultipage(t *testing.T) {
	attempts := []inspectResult{{
		NumberOfPages:        1,
		LastPageContentRatio: 0.88,
	}, {
		NumberOfPages:        1,
		LastPageContentRatio: 0.83,
	}, {
		NumberOfPages:        2,
		LastPageContentRatio: 0.95,
	}, {
		NumberOfPages:        1,
		LastPageContentRatio: 0.66,
	}}
	best := getBestAttemptIndex(attempts)
	if best != 0 {
		t.Fatalf("wrong index for best attempt")
	}
}

func TestCorrectlyPicksBetterPNGInspectResultFromSinglepage(t *testing.T) {
	attempts := []inspectResult{{
		NumberOfPages:        1,
		LastPageContentRatio: 0.88,
	}, {
		NumberOfPages:        1,
		LastPageContentRatio: 0.83,
	}, {
		NumberOfPages:        1,
		LastPageContentRatio: 0.95,
	}, {
		NumberOfPages:        1,
		LastPageContentRatio: 0.66,
	}}
	best := getBestAttemptIndex(attempts)
	if best != 2 {
		t.Fatalf("wrong index for best attempt")
	}
}
