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

type country int

type namedInt16 int16

type namedBool bool

type failingPointerText string

type failingValueText string

var errTextUnmarshal = errors.New("text unmarshaling failed")

const countryJapan country = 81

func (c *country) UnmarshalText(text []byte) error {
	if string(text) != "JP" {
		return errors.New("unknown country")
	}
	*c = countryJapan
	return nil
}

func (*failingPointerText) UnmarshalText([]byte) error {
	return errTextUnmarshal
}

func (failingValueText) UnmarshalText([]byte) error {
	return errTextUnmarshal
}

func TestParseArg_Primitives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		arg      string
		typeInfo reflect.Type
		want     any
	}{
		{name: "string", arg: "hello", typeInfo: reflect.TypeFor[string](), want: "hello"},
		{name: "bool", arg: "true", typeInfo: reflect.TypeFor[bool](), want: true},
		{name: "int", arg: "-1", typeInfo: reflect.TypeFor[int](), want: int(-1)},
		{name: "int8", arg: "-8", typeInfo: reflect.TypeFor[int8](), want: int8(-8)},
		{name: "int16", arg: "-16", typeInfo: reflect.TypeFor[int16](), want: int16(-16)},
		{name: "int32", arg: "-32", typeInfo: reflect.TypeFor[int32](), want: int32(-32)},
		{name: "int64", arg: "-64", typeInfo: reflect.TypeFor[int64](), want: int64(-64)},
		{name: "uint", arg: "1", typeInfo: reflect.TypeFor[uint](), want: uint(1)},
		{name: "uint8", arg: "8", typeInfo: reflect.TypeFor[uint8](), want: uint8(8)},
		{name: "uint16", arg: "16", typeInfo: reflect.TypeFor[uint16](), want: uint16(16)},
		{name: "uint32", arg: "32", typeInfo: reflect.TypeFor[uint32](), want: uint32(32)},
		{name: "uint64", arg: "64", typeInfo: reflect.TypeFor[uint64](), want: uint64(64)},
		{name: "uintptr", arg: "128", typeInfo: reflect.TypeFor[uintptr](), want: uintptr(128)},
		{name: "float32", arg: "3.5", typeInfo: reflect.TypeFor[float32](), want: float32(3.5)},
		{name: "float64", arg: "7.25", typeInfo: reflect.TypeFor[float64](), want: float64(7.25)},
		{name: "complex64", arg: "1.5+2.5i", typeInfo: reflect.TypeFor[complex64](), want: complex64(1.5 + 2.5i)},
		{name: "complex128", arg: "3.25-4.75i", typeInfo: reflect.TypeFor[complex128](), want: complex128(3.25 - 4.75i)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseArg(test.arg, test.typeInfo)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", test.name, err)
			}
			if got.Type() != test.typeInfo {
				t.Fatalf("expected type %v, got %v", test.typeInfo, got.Type())
			}
			if !reflect.DeepEqual(got.Interface(), test.want) {
				t.Fatalf("expected %#v, got %#v", test.want, got.Interface())
			}
		})
	}
}

func TestParseArg_PrimitiveRangeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		arg      string
		typeInfo reflect.Type
	}{
		{name: "int8 overflow", arg: "128", typeInfo: reflect.TypeFor[int8]()},
		{name: "uint8 overflow", arg: "256", typeInfo: reflect.TypeFor[uint8]()},
		{name: "negative unsigned integer", arg: "-1", typeInfo: reflect.TypeFor[uint]()},
		{name: "float32 overflow", arg: "1e100", typeInfo: reflect.TypeFor[float32]()},
		{name: "complex64 overflow", arg: "1e100+1e100i", typeInfo: reflect.TypeFor[complex64]()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseArg(test.arg, test.typeInfo)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got %v", err)
			}
		})
	}
}

func TestParseArg_NamedPrimitive(t *testing.T) {
	t.Parallel()

	got, err := parseArg("16", reflect.TypeFor[namedInt16]())
	if err != nil {
		t.Fatalf("failed to parse named primitive: %v", err)
	}
	if got.Type() != reflect.TypeFor[namedInt16]() {
		t.Fatalf("expected namedInt16 type, got %v", got.Type())
	}
	if value := got.Interface().(namedInt16); value != 16 {
		t.Fatalf("expected 16, got %d", value)
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

func TestParseArg_TextUnmarshalerTakesPriorityOverUnderlyingType(t *testing.T) {
	t.Parallel()

	val, err := parseArg("JP", reflect.TypeFor[country]())
	if err != nil {
		t.Fatalf("failed to parse named primitive type: %v", err)
	}
	if got := val.Interface().(country); got != countryJapan {
		t.Fatalf("expected countryJapan, got %d", got)
	}
}

func TestParseArg_PreservesTextUnmarshalerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typeInfo reflect.Type
	}{
		{name: "pointer receiver", typeInfo: reflect.TypeFor[failingPointerText]()},
		{name: "value receiver", typeInfo: reflect.TypeFor[failingValueText]()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseArg("invalid", test.typeInfo)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got %v", err)
			}
			if !errors.Is(err, errTextUnmarshal) {
				t.Fatalf("expected text unmarshaler error, got %v", err)
			}
		})
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

	values, err := bindArgsToValues([]string{"alice", "--count=3", "-v", "--task", "build", "test"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	if got := findValueByName(specs, values, "name").String(); got != "alice" {
		t.Fatalf("expected positional name alice, got %q", got)
	}
	if got := findValueByName(specs, values, "count").Int(); got != 3 {
		t.Fatalf("expected count 3, got %d", got)
	}
	if !findValueByName(specs, values, "verbose").Bool() {
		t.Fatalf("expected verbose true")
	}

	tasks := findValueByName(specs, values, "task").Interface().([]string)
	if !reflect.DeepEqual(tasks, []string{"build", "test"}) {
		t.Fatalf("unexpected task values: %#v", tasks)
	}
}

func TestBindArgsToValues_BindsSliceOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []country
	}{
		{name: "separated", args: []string{"--country", "JP"}, want: []country{countryJapan}},
		{name: "valued", args: []string{"--country=JP"}, want: []country{countryJapan}},
		{name: "repeated", args: []string{"--country", "JP", "--country=JP"}, want: []country{countryJapan, countryJapan}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			specs := []argSpec{{index: -1, name: "country", typeInfo: reflect.TypeFor[[]country]()}}
			values, err := bindArgsToValues(test.args, specs)
			if err != nil {
				t.Fatalf("bind failed: %v", err)
			}
			if got := values[0].Interface().([]country); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("expected %#v, got %#v", test.want, got)
			}
		})
	}
}

func TestBindArgsToValues_BoolExplicitFalse(t *testing.T) {
	t.Parallel()

	specs := []argSpec{{index: -1, name: "verbose", short: "v", typeInfo: reflect.TypeFor[bool]()}}

	values, err := bindArgsToValues([]string{"--verbose=false"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	if !values[0].IsValid() {
		t.Fatalf("expected bool value to be set")
	}
	if values[0].Bool() {
		t.Fatalf("expected explicit false value")
	}
}

func TestBindArgsToValues_NamedBoolFlag(t *testing.T) {
	t.Parallel()

	specs := []argSpec{{index: -1, name: "enabled", typeInfo: reflect.TypeFor[namedBool]()}}

	values, err := bindArgsToValues([]string{"--enabled"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	if values[0].Type() != reflect.TypeFor[namedBool]() {
		t.Fatalf("expected namedBool type, got %v", values[0].Type())
	}
	if !values[0].Bool() {
		t.Fatal("expected enabled flag to be true")
	}
}

func TestBindArgsToValues_MissingOptionValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		typeInfo reflect.Type
	}{
		{name: "scalar", args: []string{"--count"}, typeInfo: reflect.TypeFor[int]()},
		{name: "before another option", args: []string{"--count", "--help"}, typeInfo: reflect.TypeFor[int]()},
		{name: "slice", args: []string{"--count"}, typeInfo: reflect.TypeFor[[]int]()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			specs := []argSpec{
				{index: -1, name: "count", short: "c", typeInfo: test.typeInfo},
				helpArgSpec,
			}
			_, err := bindArgsToValues(test.args, specs)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got: %v", err)
			}
			if !strings.Contains(err.Error(), "missing value for option: count") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBindArgsToValues_ParsesOptionsStrictly(t *testing.T) {
	t.Parallel()

	specs := []argSpec{
		{index: -1, name: "count", short: "c", typeInfo: reflect.TypeFor[int]()},
		{index: -1, name: "verbose", short: "v", typeInfo: reflect.TypeFor[bool]()},
	}

	tests := []struct {
		name string
		arg  string
	}{
		{name: "unknown long option", arg: "--missing"},
		{name: "unknown short option", arg: "-x"},
		{name: "long name with short prefix", arg: "-count"},
		{name: "short name with long prefix", arg: "--c"},
		{name: "too many hyphens", arg: "---count"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := bindArgsToValues([]string{test.arg}, specs)
			if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "unknown option") {
				t.Fatalf("expected unknown option error, got %v", err)
			}
		})
	}
}

func TestBindArgsToValues_OptionTerminator(t *testing.T) {
	t.Parallel()

	specs := []argSpec{
		{index: 0, name: "value", typeInfo: reflect.TypeFor[string]()},
		{index: -1, name: "verbose", short: "v", typeInfo: reflect.TypeFor[bool]()},
	}
	values, err := bindArgsToValues([]string{"--", "--verbose"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	if got := findValueByName(specs, values, "value").String(); got != "--verbose" {
		t.Fatalf("expected literal positional value, got %q", got)
	}
	if findValueByName(specs, values, "verbose").IsValid() {
		t.Fatal("expected option parsing to stop after --")
	}
}

func TestBindArgsToValues_NegativeNumbers(t *testing.T) {
	t.Parallel()

	specs := []argSpec{
		{index: 0, name: "offset", typeInfo: reflect.TypeFor[int]()},
		{index: -1, name: "values", short: "v", typeInfo: reflect.TypeFor[[]float64]()},
	}
	values, err := bindArgsToValues([]string{"-2", "--values", "-1.5", "2.5"}, specs)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	if got := findValueByName(specs, values, "offset").Int(); got != -2 {
		t.Fatalf("expected -2, got %d", got)
	}
	if got := findValueByName(specs, values, "values").Interface(); !reflect.DeepEqual(got, []float64{-1.5, 2.5}) {
		t.Fatalf("unexpected values: %#v", got)
	}
}

func TestBindArgsToValues_NegativeUnsignedIntegerIsAValueError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		specs []argSpec
	}{
		{
			name:  "positional",
			args:  []string{"-1"},
			specs: []argSpec{{index: 0, name: "count", typeInfo: reflect.TypeFor[uint]()}},
		},
		{
			name:  "option",
			args:  []string{"--count", "-1"},
			specs: []argSpec{{index: -1, name: "count", typeInfo: reflect.TypeFor[uint]()}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := bindArgsToValues(test.args, test.specs)
			if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "expected an unsigned integer") {
				t.Fatalf("expected unsigned integer parse error, got %v", err)
			}
		})
	}
}

func TestBindArgsToValues_PreservesParseArgError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		specs []argSpec
	}{
		{
			name:  "positional argument",
			args:  []string{"invalid"},
			specs: []argSpec{{index: 0, name: "value", typeInfo: reflect.TypeFor[failingPointerText]()}},
		},
		{
			name:  "valued option",
			args:  []string{"--value=invalid"},
			specs: []argSpec{{index: -1, name: "value", typeInfo: reflect.TypeFor[failingPointerText]()}},
		},
		{
			name:  "option argument",
			args:  []string{"--value", "invalid"},
			specs: []argSpec{{index: -1, name: "value", typeInfo: reflect.TypeFor[failingPointerText]()}},
		},
		{
			name:  "slice option argument",
			args:  []string{"--value", "invalid"},
			specs: []argSpec{{index: -1, name: "value", typeInfo: reflect.TypeFor[[]failingPointerText]()}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := bindArgsToValues(test.args, test.specs)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected ErrInvalidArgument, got %v", err)
			}
			if !errors.Is(err, errTextUnmarshal) {
				t.Fatalf("expected parse argument error, got %v", err)
			}
		})
	}
}

func TestBindArgsToValues_ResetsPreviousValues(t *testing.T) {
	t.Parallel()

	specs := []argSpec{
		{index: 0, name: "name", typeInfo: reflect.TypeFor[string]()},
		{index: -1, name: "count", short: "c", typeInfo: reflect.TypeFor[int]()},
	}
	sortArgSpecs(specs)

	firstValues, err := bindArgsToValues([]string{"alice", "--count", "9"}, specs)
	if err != nil {
		t.Fatalf("first bind failed: %v", err)
	}

	secondValues, err := bindArgsToValues([]string{"bob"}, specs)
	if err != nil {
		t.Fatalf("second bind failed: %v", err)
	}

	if got := findValueByName(specs, secondValues, "name").String(); got != "bob" {
		t.Fatalf("expected second positional value bob, got %q", got)
	}
	if findValueByName(specs, secondValues, "count").IsValid() {
		t.Fatalf("expected optional count to be reset between runs")
	}
	if got := findValueByName(specs, firstValues, "count").Int(); got != 9 {
		t.Fatalf("expected first bind to remain isolated, got %d", got)
	}
}

func findValueByName(specs []argSpec, values []reflect.Value, name string) reflect.Value {
	for i := range specs {
		if specs[i].name == name {
			return values[i]
		}
	}
	return reflect.Value{}
}

func findSpecByName(specs []argSpec, name string) *argSpec {
	for i := range specs {
		if specs[i].name == name {
			return &specs[i]
		}
	}
	return nil
}
