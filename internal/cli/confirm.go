package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// confirmDestructive prints a summary of an irreversible operation and asks
// the operator to approve it. It returns false when the answer is anything
// other than "y", including on EOF, so a closed stdin aborts the command
// instead of approving it.
func confirmDestructive(in io.Reader, out io.Writer, header string, lines []string) (bool, error) {
	fmt.Fprintf(out, "\n=== %s ===\n", header)
	for _, line := range lines {
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "\nType 'y' to confirm, anything else to abort:")

	input, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(input)) == "y", nil
}
