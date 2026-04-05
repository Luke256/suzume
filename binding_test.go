package suzume

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type lowerText struct {
	value string
}

func (l *lowerText) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return errors.New("empty value")
	}
	*l = lowerText{value: strings.ToLower(string(text))}
	return nil
}

func TestParseArg_Primitives(t *testing.T) {
	t.Parallel()

	intVal, err := parseArg("42", reflect.TypeFor[int]())
	if err != nil {
		t.Fatalf("failed to parse int: %v", err)
	}
	if intVal.Int() != 42 {
		t.Fatalf("expected int 42, got %d", intVal.Int())
	}

	floatVal, err := parseArg("3.14", reflect.TypeFor[float64]())
	if err != nil {
		t.Fatalf("failed to parse float: %v", err)
	}
	if floatVal.Float() != 3.14 {
		t.Fatalf("expected float 3.14, got %v", floatVal.Float())
	}

	boolVal, err := parseArg("true", reflect.TypeFor[bool]())
	if err != nil {
		t.Fatalf("failed to parse bool: %v", err)
	}
	if !boolVal.Bool() {
		t.Fatalf("expected bool true")
	}
}

func TestParseArg_TextUnmarshaler(t *testing.T) {
	t.Parallel()

	val, err := parseArg("HeLLo", reflect.TypeFor[lowerText]())
	if err != nil {
		t.Fatalf("failed to parse custom text type: %v", err)
	}
	if got := val.Interface().(lowerText); got.value != "hello" {
		t.Fatalf("expected lowerText.value to be hello, got %q", got.value)
	}
}

func TestBindArgsToValues_BindsPositionalAndOptions(t *testing.T) {
	t.Parallel()

	specs := []argSpec{
		{index: 0, name: "name", typeInfo: reflect.TypeFor[string]()},
		{index: -1, name: "count", short: "c", typeInfo: reflect.TypeFor[int]()},
		{index: -1, name: "verbose", short: "v", typeInfo: reflect.TypeFor[bool]()},
		{index: -1, name: "task", short: "t", typeInfo: reflect.TypeFor[[]string]()},
	}
	sortArgSpecs(specs)

	err := bindArgsToValues([]string{"alice", "--count=3", "-v", "--task", "build", "test"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	if got := findSpecByName(specs, "name").value.String(); got != "alice" {
		t.Fatalf("expected positional name alice, got %q", got)
	}
	if got := findSpecByName(specs, "count").value.Int(); got != 3 {
		t.Fatalf("expected count 3, got %d", got)
	}
	if !findSpecByName(specs, "verbose").value.Bool() {
		t.Fatalf("expected verbose true")
	}

	tasks := findSpecByName(specs, "task").value.Interface().([]string)
	if !reflect.DeepEqual(tasks, []string{"build", "test"}) {
		t.Fatalf("unexpected task values: %#v", tasks)
	}
}

func TestBindArgsToValues_BoolExplicitFalse(t *testing.T) {
	t.Parallel()

	specs := []argSpec{{index: -1, name: "verbose", short: "v", typeInfo: reflect.TypeFor[bool]()}}

	err := bindArgsToValues([]string{"--verbose=false"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	if !specs[0].value.IsValid() {
		t.Fatalf("expected bool value to be set")
	}
	if specs[0].value.Bool() {
		t.Fatalf("expected explicit false value")
	}
}

func TestBindArgsToValues_MissingOptionValue(t *testing.T) {
	t.Parallel()

	specs := []argSpec{{index: -1, name: "count", short: "c", typeInfo: reflect.TypeFor[int]()}}

	err := bindArgsToValues([]string{"--count"}, specs)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got: %v", err)
	}
	if !strings.Contains(err.Error(), "missing value for option: count") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgsToValues_ResetsPreviousValues(t *testing.T) {
	t.Parallel()

	specs := []argSpec{
		{index: 0, name: "name", typeInfo: reflect.TypeFor[string]()},
		{index: -1, name: "count", short: "c", typeInfo: reflect.TypeFor[int]()},
	}
	sortArgSpecs(specs)

	err := bindArgsToValues([]string{"alice", "--count", "9"}, specs)
	if err != nil {
		t.Fatalf("first bind failed: %v", err)
	}

	err = bindArgsToValues([]string{"bob"}, specs)
	if err != nil {
		t.Fatalf("second bind failed: %v", err)
	}

	if got := findSpecByName(specs, "name").value.String(); got != "bob" {
		t.Fatalf("expected second positional value bob, got %q", got)
	}
	if findSpecByName(specs, "count").value.IsValid() {
		t.Fatalf("expected optional count to be reset between runs")
	}
}

func findSpecByName(specs []argSpec, name string) *argSpec {
	for i := range specs {
		if specs[i].name == name {
			return &specs[i]
		}
	}
	return nil
}
