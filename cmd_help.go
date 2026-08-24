package suzume

import (
	"fmt"
	"reflect"
)

func (cmd *Executable) showHelp() {
	var numArguments int
	var numOptions int
	var out = cmd.config.Log
	var defaultValues map[string]any
	if cmd.defaultValues != nil {
		defaultValues = cmd.defaultValues()
	}

	fmt.Fprintf(out, "Usage: %s", cmd.name)
	for _, arg := range cmd.argSpecs {
		if arg.index >= 0 {
			fmt.Fprintf(out, " <%s>", arg.name)
			numArguments++
		} else {
			if arg.short != "" {
				fmt.Fprintf(out, " [-%s|--%s]", arg.short, arg.name)
			} else if arg.name != "" {
				fmt.Fprintf(out, " [--%s]", arg.name)
			}
			numOptions++
		}
	}
	fmt.Fprintln(out)

	if cmd.description != "" {
		fmt.Fprintln(out, cmd.description)
	}

	if numArguments > 0 {
		fmt.Fprintln(out, "\nArguments:")
		for _, arg := range cmd.argSpecs {
			if arg.index >= 0 {
				fmt.Fprintf(out, "  %s\t%s\n", arg.name, arg.usage)
			}
		}
	}

	if numOptions > 0 {
		fmt.Fprintln(out, "\nOptions:")
		for _, arg := range cmd.argSpecs {
			if arg.index == optionsIndex {
				usage := arg.usage
				defaultText, showDefault := optionDefaultText(arg, defaultValues)
				if showDefault {
					if usage != "" {
						usage += " "
					}
					usage += fmt.Sprintf("(default: %s)", defaultText)
				}

				if arg.short != "" {
					fmt.Fprintf(out, "  -%s, --%s\t%s\n", arg.short, arg.name, usage)
				} else if arg.name != "" {
					fmt.Fprintf(out, "      --%s\t%s\n", arg.name, usage)
				}
			}
		}
	}
}

func optionDefaultText(arg argSpec, defaultValues map[string]any) (string, bool) {
	if arg.typeInfo.Kind() == reflect.Bool {
		return "", false
	}
	if arg.hasDefaultText {
		return arg.defaultText, arg.defaultText != ""
	}

	defaultValue, ok := defaultValues[arg.fieldName]
	if !ok {
		return "", false
	}
	return fmt.Sprint(defaultValue), true
}
