package api

import "encoding/json"

// jsonUnmarshal is a convenience wrapper around json.Unmarshal.
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
