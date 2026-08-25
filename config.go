package suzume

import (
	"io"
	"os"
)

// config contains the runtime settings for an application or command.
// The concrete type is intentionally private so configuration is created through
// DefaultConfig or NewConfig.
type config struct {
	log           io.Writer
	errorLog      io.Writer
	ignoreSignals []os.Signal
}

// ConfigOption configures a value created by NewConfig.
// Options are provided by this package through the With* functions.
type ConfigOption interface {
	apply(*config)
}

type configOptionFunc func(*config)

func (option configOptionFunc) apply(configuration *config) {
	option(configuration)
}

// DefaultConfig returns a configuration that writes normal output to os.Stdout,
// writes errors to os.Stderr, and does not intercept any signals.
func DefaultConfig() config {
	return config{
		log:      os.Stdout,
		errorLog: os.Stderr,
	}
}

func materializeConfig(configuration *config) config {
	if configuration != nil {
		return *configuration
	}
	return DefaultConfig()
}

// NewConfig creates a default configuration and applies options in order.
// Nil options are ignored.
func NewConfig(options ...ConfigOption) config {
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
	return configOptionFunc(func(configuration *config) {
		if writer != nil {
			configuration.log = writer
		}
	})
}

// WithErrorLog sets the destination for error output.
// A nil writer leaves the default destination unchanged.
func WithErrorLog(writer io.Writer) ConfigOption {
	return configOptionFunc(func(configuration *config) {
		if writer != nil {
			configuration.errorLog = writer
		}
	})
}

// WithIgnoreSignals sets the signals that cancel a command's context instead of
// using the process's default signal behavior.
func WithIgnoreSignals(signals ...os.Signal) ConfigOption {
	configuredSignals := append([]os.Signal(nil), signals...)
	return configOptionFunc(func(configuration *config) {
		configuration.ignoreSignals = append([]os.Signal(nil), configuredSignals...)
	})
}
