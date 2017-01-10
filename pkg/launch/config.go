package launch

import (
	"encoding/json"
)

// Config is the raw configuration on how to launch the plugin.
type Config json.RawMessage

// Unmarshal decodes the raw config container in this object to the typed struct.
func (c *Config) Unmarshal(typed interface{}) error {
	if c == nil || len([]byte(*c)) == 0 {
		return nil // no effect on typed
	}
	return json.Unmarshal([]byte(*c), typed)
}

// Marshal populates this raw message with a decoded form of the input struct.
func (c *Config) Marshal(typed interface{}) error {
	buff, err := json.MarshalIndent(typed, "", "  ")
	if err != nil {
		return err
	}
	*c = Config(json.RawMessage(buff))
	return nil
}

// String returns the string representation.
func (c *Config) String() string {
	return string([]byte(*c))
}

func (c *Config) MarshalJSON() ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	return []byte(*c), nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	*c = Config(json.RawMessage(data))
	return nil
}
