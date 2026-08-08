package httpapi

import "testing"

func TestGitHubHelpers(t *testing.T) {
	payload := []byte(`{"zen":"correct horse battery staple"}`)
	if !validGitHubSignature(payload, "sha256=40d2613f13b782e2a8070ba0db064b95cc2e3e5f7c32ffbfab989f4e2e5e17d9", "secret") {
		t.Fatal("valid signature rejected")
	}
	if validGitHubSignature(payload, "sha256=deadbeef", "secret") {
		t.Fatal("invalid signature accepted")
	}
	if repositoryKey("FreshCode/CRM FreshCode") != "crm-freshcode" {
		t.Fatalf("unexpected repository key")
	}
	for input, expected := range map[string]string{"success": "success", "timed_out": "failure", "skipped": "cancelled", "requested": "queued"} {
		if actual := normalizeDeploymentStatus(input); actual != expected {
			t.Fatalf("status %q = %q, want %q", input, actual, expected)
		}
	}
}
