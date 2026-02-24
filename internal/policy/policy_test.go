package policy

import (
	"testing"

	"db-snap/internal/model"
)

func TestValidateTargetLoopbackAllowed(t *testing.T) {
	res, err := ValidateTarget("localhost", model.SafetyPolicy{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.Allowed {
		t.Fatalf("expected allowed")
	}
	if res.RequiresWarningAck {
		t.Fatalf("expected no warning ack for loopback")
	}
}

func TestValidateTargetPrivateAllowlist(t *testing.T) {
	res, err := ValidateTarget("10.0.0.5", model.SafetyPolicy{AllowCIDRs: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.RequiresWarningAck {
		t.Fatalf("expected warning ack for private target")
	}
}

func TestValidateTargetPublicDenied(t *testing.T) {
	_, err := ValidateTarget("8.8.8.8", model.SafetyPolicy{AllowCIDRs: []string{"10.0.0.0/8"}})
	if err == nil {
		t.Fatalf("expected error for public ip")
	}
}

func TestValidateTargetKeywordDenied(t *testing.T) {
	_, err := ValidateTarget("prod-db.local", model.SafetyPolicy{DenyHostKeywords: []string{"prod"}})
	if err == nil {
		t.Fatalf("expected deny keyword error")
	}
}
