package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SafeMap is a map[string]string that handles null values gracefully.
// Used for Labels, Notes, DefaultAddresses, FeeLimits, Shares, and Query.
type SafeMap map[string]string

// UnmarshalJSON implements custom unmarshalling for SafeMap.
func (m *SafeMap) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	result := make(SafeMap, len(raw))
	for k, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			// If it's not a string, store the raw JSON
			result[k] = strings.Trim(string(v), `"`)
		} else {
			result[k] = s
		}
	}
	*m = result
	return nil
}

// SafeMapAny is a map[string]interface{} for data filters and similar.
type SafeMapAny map[string]interface{}

// Set is a collection of unique strings, serialized as a JSON array.
type Set []string

// DurationString represents a duration string like "10d", "3h", "30m".
type DurationString string

// FlexibleArray can be either a single value or an array.
// When serializing, a single value is output as-is, and multiple values as an array.
// When deserializing, both a single string and an array are accepted.
type FlexibleArray []string

// UnmarshalJSON implements custom unmarshalling for FlexibleArray.
func (f *FlexibleArray) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = nil
		return nil
	}
	// Try as array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = arr
		return nil
	}
	// Try as single string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleArray{s}
		return nil
	}
	return fmt.Errorf("FlexibleArray: cannot unmarshal %s", string(data))
}

// MarshalJSON implements custom marshalling for FlexibleArray.
func (f FlexibleArray) MarshalJSON() ([]byte, error) {
	if len(f) == 1 {
		return json.Marshal(f[0])
	}
	return json.Marshal([]string(f))
}

// FlexibleResourceFilter can be a ResourceType string, an array, or a structured filter object.
type FlexibleResourceFilter struct {
	// For simple string or array form
	Types []string `json:"-"`
	// For structured form
	Type    *string `json:"type,omitempty"`
	State   *string `json:"state,omitempty"`
	Variant *string `json:"variant,omitempty"`
	Name    *string `json:"name,omitempty"`
	Tag     *string `json:"tag,omitempty"`
}

// UnmarshalJSON implements custom unmarshalling for FlexibleResourceFilter.
func (f *FlexibleResourceFilter) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	// Try as single string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		f.Types = []string{s}
		return nil
	}
	// Try as array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		f.Types = arr
		return nil
	}
	// Try as structured object
	type Alias FlexibleResourceFilter
	var a Alias
	if err := json.Unmarshal(data, &a); err == nil {
		*f = FlexibleResourceFilter(a)
		return nil
	}
	return fmt.Errorf("FlexibleResourceFilter: cannot unmarshal %s", string(data))
}

// MarshalJSON implements custom marshalling for FlexibleResourceFilter.
func (f FlexibleResourceFilter) MarshalJSON() ([]byte, error) {
	if len(f.Types) > 0 {
		if len(f.Types) == 1 {
			return json.Marshal(f.Types[0])
		}
		return json.Marshal(f.Types)
	}
	type Alias FlexibleResourceFilter
	return json.Marshal(Alias(f))
}

// ParseResourceId extracts the ID from a resource name like "accounts/foo" -> "foo".
func ParseResourceId(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return name
}

// ParseResourcePrefix extracts the prefix from a resource name like "accounts/foo" -> "accounts".
func ParseResourcePrefix(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}
