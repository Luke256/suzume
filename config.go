package suzume

import (
	"io"
	"os"
)

// Config represents the configuration for an application or command, including settings for log output and error output.
type Config struct {
	// inherit indicates whether the configuration should be inherited from the parent application.
	// If true, the application or command will use the configuration of its parent application unless it is overridden by its own configuration.
	inherit bool

	// Log is the destination for log output. By default, it is set to os.Stdout.
	Log io.Writer

	// ErrorLog is the destination for error output. By default, it is set to os.Stderr.
	ErrorLog io.Writer
}

func defaultConfig() Config {
	return Config{
		inherit:  true,
		Log:      os.Stdout,
		ErrorLog: os.Stderr,
	}
}