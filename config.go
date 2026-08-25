package suzume

import (
	"io"
	"os"
)

// Config contains the runtime settings for an application or command.
// Create values with DefaultConfig or NewConfig.
type Config struct {
	log           io.Writer
	errorLog      io.Writer
	ignoreSignals []os.Signal
}

// ConfigOption configures a value created by NewConfig.
// Options are provided by this package through the With* functions.
type ConfigOption interface {
	apply(*Config)
}

type configOptionFunc func(*Config)

func (option configOptionFunc) apply(configuration *Config) {
	option(configuration)
}

// DefaultConfig returns a Config that writes normal output to os.Stdout,
// writes errors to os.Stderr, and does not intercept any signals.
func DefaultConfig() Config {
	return Config{
		log:      os.Stdout,
		errorLog: os.Stderr,
	}
}

func materializeConfig(configuration *Config) Config {
	if configuration != nil {
		return *configuration
	}
	return DefaultConfig()
}

// NewConfig creates a Config with default settings and applies options in order.
// Nil options are ignored.
func NewConfig(options ...ConfigOption) Config {
	configuration := DefaultConfig()
	for _, option := range options {
		if option != nil {
			option.apply(&configuration)
		}
	}
	return configuration
}

// WithLog sets the destination for normal output.
// A nil writer leaves the default destination unchanged.
func WithLog(writer io.Writer) ConfigOption {
	return configOptionFunc(func(configuration *Config) {
		if writer != nil {
			configuration.log = writer
		}
	})
}

// WithErrorLog sets the destination for error output.
// A nil writer leaves the default destination unchanged.
func WithErrorLog(writer io.Writer) ConfigOption {
	return configOptionFunc(func(configuration *Config) {
		if writer != nil {
			configuration.errorLog = writer
		}
	})
}

// WithIgnoreSignals sets the signals that cancel a command's context instead of
// using the process's default signal behavior.
func WithIgnoreSignals(signals ...os.Signal) ConfigOption {
	configuredSignals := append([]os.Signal(nil), signals...)
	return configOptionFunc(func(configuration *Config) {
		configuration.ignoreSignals = append([]os.Signal(nil), configuredSignals...)
	})
}
