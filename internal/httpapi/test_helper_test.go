package httpapi

import (
	"encoding/json"
	"testing"
)

func testJSON(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("invalid JSON: %s: %v", body, err)
	}
	return v
}
