package suzume

import (
	"fmt"
)

func (cmd *Command) showHelp() {
	var numArguments int
	var numOptions int
	var out = cmd.config.Log

	fmt.Fprintf(out, "Usage: %s", cmd.name)
	for _, arg := range cmd.argSpecs {
		if arg.index != -1 {
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
			if arg.index != -1 {
				fmt.Fprintf(out, "  %s\t%s\n", arg.name, arg.usage)
			}
		}
	}

	if numOptions > 0 {
		fmt.Fprintln(out, "\nOptions:")
		for _, arg := range cmd.argSpecs {
			if arg.index == -1 {
				if arg.short != "" {
					fmt.Fprintf(out, "  -%s, --%s\t%s\n", arg.short, arg.name, arg.usage)
				} else if arg.name != "" {
					fmt.Fprintf(out, "      --%s\t%s\n", arg.name, arg.usage)
				}
			}
		}
	}
}