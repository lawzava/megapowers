// Package strictjson reads configuration without ambiguous keys or extra data.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func ReadFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := Decode(data, target); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func Decode(data []byte, target any) error {
	keys := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueValue(keys); err != nil {
		return err
	}
	if _, err := keys.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func uniqueValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key must be a string")
			}
			if seen[name] {
				return fmt.Errorf("duplicate JSON key %q", name)
			}
			seen[name] = true
			if err := uniqueValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	_, err = decoder.Token()
	return err
}
