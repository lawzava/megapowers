package strictjson

import "testing"

func TestDecode(t *testing.T) {
	for _, tc := range []struct {
		input string
		valid bool
	}{
		{`{"name":"ok"}`, true},
		{`{"name":"ok"} `, true},
		{`{"extra":1}`, false},
		{`{"name":"ok"} {}`, false},
		{`{"name":"a","name":"b"}`, false},
		{`{"name":"a","na\u006de":"b"}`, false},
		{``, false},
	} {
		var value struct {
			Name string `json:"name"`
		}
		err := Decode([]byte(tc.input), &value)
		if (err == nil) != tc.valid {
			t.Errorf("Decode(%q): %v", tc.input, err)
		}
	}
}
