// Package testdata contains fixtures for editor tests.
package testdata

// Config holds application settings.
type Config struct {
	// Host is the server hostname.
	Host string
	// Port is the listening port.
	Port int
	// Timeout in seconds.
	Timeout int
}

// NewConfig returns a default Config.
func NewConfig() Config {
	return Config{
		Host:    "localhost",
		Port:    8080,
		Timeout: 30,
	}
}

// Validate checks that the config is valid.
func Validate(c Config) bool {
	return c.Port > 0
}
