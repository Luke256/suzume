package suzume

import (
	"fmt"
)

func (cmd *Command) showHelp() {
	var numArguments int
	var numOptions int

	fmt.Printf("Usage: %s", cmd.name)
	for _, arg := range cmd.argSpecs {
		if arg.index != -1 {
			fmt.Printf(" <%s>", arg.name)
			numArguments++
		} else {
			if arg.short != "" {
				fmt.Printf(" [-%s|--%s]", arg.short, arg.name)
			} else if arg.name != "" {
				fmt.Printf(" [--%s]", arg.name)
			}
			numOptions++
		}
	}
	fmt.Println()

	if cmd.description != "" {
		fmt.Println(cmd.description)
	}

	if numArguments > 0 {
		fmt.Println("\nArguments:")
		for _, arg := range cmd.argSpecs {
			if arg.index != -1 {
				fmt.Printf("  %s\t%s\n", arg.name, arg.usage)
			}
		}
	}

	if numOptions > 0 {
		fmt.Println("\nOptions:")
		for _, arg := range cmd.argSpecs {
			if arg.index == -1 {
				if arg.short != "" {
					fmt.Printf("  -%s, --%s\t%s\n", arg.short, arg.name, arg.usage)
				} else if arg.name != "" {
					fmt.Printf("      --%s\t%s\n", arg.name, arg.usage)
				}
			}
		}
	}
}