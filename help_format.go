package suzume

import (
	"fmt"
	"io"
	"text/tabwriter"
)

type helpItem struct {
	label       string
	description string
}

func writeHelpItems(out io.Writer, items []helpItem) {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, item := range items {
		fmt.Fprintf(writer, "  %s\t%s\n", item.label, item.description)
	}
	_ = writer.Flush()
}
