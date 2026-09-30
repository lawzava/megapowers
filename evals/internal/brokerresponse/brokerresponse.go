// Package brokerresponse checks sandbox-broker output before a study decodes
// it: encoding/json turns an omitted or null field into a zero value, which
// would grade an incomplete run as a clean one.
package brokerresponse

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RequirePresent fails when any named top-level field is missing or null.
func RequirePresent(content []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(content, &object); err != nil {
		return fmt.Errorf("sandbox broker response: %w", err)
	}
	for _, field := range fields {
		value, present := object[field]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("sandbox broker response omitted required field %q", field)
		}
	}
	return nil
}
