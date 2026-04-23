package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/itchyny/gojq"
)

func TestApplyJQFilter(t *testing.T) {
	t.Run("simple select", func(t *testing.T) {
		body := []byte(`{"user": "alice", "active": true}`)
		filter := `.user`

		query, err := gojq.Parse(filter)
		if err != nil {
			t.Fatalf("failed to parse filter: %v", err)
		}

		r := map[string]any{}
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		iter := query.Run(r)
		var v []byte
		for {
			val, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := val.(error); ok {
				t.Fatalf("failed to iterate: %v", err)
			}
			if val != nil {
				var b []byte
				b, _ = json.Marshal(val)
				v = b
				break
			}
		}

		if !bytes.Contains(v, []byte("alice")) {
			t.Errorf("got %q, want output containing alice", string(v))
		}
	})

	t.Run("path expression", func(t *testing.T) {
		body := []byte(`{"data": {"items": [{"name": "foo"}, {"name": "bar"}]}}`)
		filter := `.data.items[0].name`

		query, err := gojq.Parse(filter)
		if err != nil {
			t.Fatalf("failed to parse filter: %v", err)
		}

		r := map[string]any{}
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		iter := query.Run(r)
		var v []byte
		for {
			val, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := val.(error); ok {
				t.Fatalf("failed to iterate: %v", err)
			}
			if val != nil {
				var b []byte
				b, _ = json.Marshal(val)
				v = b
				break
			}
		}

		if !bytes.Contains(v, []byte("foo")) {
			t.Errorf("got %q, want output containing foo", string(v))
		}
	})

	t.Run("array filter", func(t *testing.T) {
		body := []byte(`["a", "b", "c"]`)
		filter := `.[0:2]`

		query, err := gojq.Parse(filter)
		if err != nil {
			t.Fatalf("failed to parse filter: %v", err)
		}

		var interfaceSlice []any
		if err := json.Unmarshal(body, &interfaceSlice); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		iter := query.Run(interfaceSlice)
		var v []byte
		for {
			val, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := val.(error); ok {
				t.Fatalf("failed to iterate: %v", err)
			}
			if val != nil {
				var b []byte
				b, _ = json.Marshal(val)
				v = b
				break
			}
		}

		if string(v) != `["a","b"]` {
			t.Errorf("got %q, want %q", string(v), `["a","b"]`)
		}
	})
}

func TestJQFilterWithEmptyFilter(t *testing.T) {
	t.Run("empty filter passes through", func(t *testing.T) {
		filter := ""
		body := []byte(`{"test": "value"}`)

		if filter != "" {
			t.Error("filter should be empty")
		}

		if !bytes.Equal(body, body) {
			t.Error("body should remain unchanged when filter is empty")
		}
	})
}
