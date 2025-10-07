package middleware

import (
	"testing"
)

func TestContextKey(t *testing.T) {
	// Test that context keys are unique strings
	if ContextKeyTenant == ContextKeyUserID {
		t.Error("ContextKeyTenant and ContextKeyUserID should be different")
	}
	
	if string(ContextKeyTenant) == "" {
		t.Error("ContextKeyTenant should not be empty")
	}
	
	if string(ContextKeyUserID) == "" {
		t.Error("ContextKeyUserID should not be empty")
	}
}
