package suzume

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrDuplicateIdentifier indicates that an identifier is already registered.
	ErrDuplicateIdentifier = errors.New("duplicated identifier")

	// ErrInvalidIdentifier indicates that an identifier is invalid.
	ErrInvalidIdentifier = errors.New("invalid identifier")
)

func validateIdentifier(identifier string) error {
	if strings.HasPrefix(identifier, "-") {
		return fmt.Errorf("%w: identifier %q cannot start with '-'", ErrInvalidIdentifier, identifier)
	}
	return nil
}

func registerIdentifiers(identifiers map[string]struct{}, name string, aliases ...string) error {
	pending := make(map[string]struct{}, len(aliases)+1)
	if _, exists := identifiers[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateIdentifier, name)
	}
	pending[name] = struct{}{}

	for _, alias := range aliases {
		if _, exists := identifiers[alias]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateIdentifier, alias)
		}
		if _, exists := pending[alias]; exists {
			continue
		}
		pending[alias] = struct{}{}
	}
	for identifier := range pending {
		identifiers[identifier] = struct{}{}
	}
	return nil
}
