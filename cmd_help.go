package suzume

import (
	"fmt"
	"reflect"
)

func (cmd *Executable) showHelp(configuration Config, commandPath string) {
	var out = configuration.log
	var defaultValues map[string]any
	if cmd.defaultValues != nil {
		defaultValues = cmd.defaultValues()
	}

	var argumentItems []helpItem
	var optionItems []helpItem

	fmt.Fprintf(out, "Usage: %s", commandPath)
	for _, arg := range cmd.argSpecs {
		if arg.index >= 0 {
			fmt.Fprintf(out, " <%s>", arg.name)
			argumentItems = append(argumentItems, helpItem{
				label:       arg.name,
				description: arg.usage,
			})
			continue
		}

		value := ""
		if arg.typeInfo.Kind() == reflect.Slice {
			value = " <value...>"
		} else if arg.typeInfo.Kind() != reflect.Bool {
			value = " <value>"
		}

		if arg.short != "" {
			fmt.Fprintf(out, " [-%s|--%s%s]", arg.short, arg.name, value)
		} else if arg.name != "" {
			fmt.Fprintf(out, " [--%s%s]", arg.name, value)
		}

		if arg.index != optionsIndex {
			continue
		}

		usage := arg.usage
		defaultText, showDefault := optionDefaultText(arg, defaultValues)
		if showDefault {
			if usage != "" {
				usage += " "
			}
			usage += fmt.Sprintf("(default: %s)", defaultText)
		}

		if arg.short != "" {
			optionItems = append(optionItems, helpItem{
				label:       fmt.Sprintf("-%s, --%s", arg.short, arg.name),
				description: usage,
			})
		} else if arg.name != "" {
			optionItems = append(optionItems, helpItem{
				label:       fmt.Sprintf("    --%s", arg.name),
				description: usage,
			})
		}
	}
	fmt.Fprintln(out)

	if cmd.description != "" {
		fmt.Fprintln(out, cmd.description)
	}

	if len(argumentItems) > 0 {
		fmt.Fprintln(out, "\nArguments:")
		writeHelpItems(out, argumentItems)
	}

	if len(optionItems) > 0 {
		fmt.Fprintln(out, "\nOptions:")
		writeHelpItems(out, optionItems)
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
