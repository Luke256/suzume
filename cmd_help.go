package suzume

import (
	"fmt"
	"sort"
)

func (cmd *Command) showHelp() {
	specs := cmd.sortedArgSpecs()

	var numArguments int
	var numOptions int

	fmt.Printf("Usage: %s", cmd.name)
	for _, arg := range specs {
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
		for _, arg := range specs {
			if arg.index != -1 {
				fmt.Printf("  %s\t%s\n", arg.name, arg.usage)
			}
		}
	}

	if numOptions > 0 {
		fmt.Println("\nOptions:")
		for _, arg := range specs {
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

func (cmd *Command) sortedArgSpecs() []argSpec {
	specs := append([]argSpec(nil), cmd.argSpecs...)
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].index == -1 {
			return false
		}
		if specs[j].index == -1 {
			return true
		}
		return specs[i].index < specs[j].index
	})
	return specs
}
