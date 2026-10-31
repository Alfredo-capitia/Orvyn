package apply

import (
	"fmt"
	"io"
	"os"
)

// PrintRecovery writes the recovery report to stdout.
func PrintRecovery(s *Session) {
	PrintRecoveryTo(os.Stdout, s)
}

// PrintRecoveryTo writes the recovery report to w.
func PrintRecoveryTo(w io.Writer, s *Session) {
	fmt.Fprintln(w, "\n=== PARTIAL FAILURE - RECOVERY ===")
	fmt.Fprintf(w, "Session: %s (started: %s, status: %s)\n", s.ID, s.StartedAt.Format("2006-01-02 15:04:05"), s.Status)
	if len(s.Created) > 0 {
		fmt.Fprintln(w, "\nCreated in this session:")
		for _, p := range s.Created {
			fmt.Fprintf(w, "  - %s\n", p)
		}
	}
	if len(s.Steps) > 0 {
		fmt.Fprintln(w, "\nSteps:")
		for _, st := range s.Steps {
			fmt.Fprintf(w, "  [%s] %s %s\n", st.Status, st.Name, st.Path)
			if st.Err != "" {
				fmt.Fprintf(w, "       error: %s\n", st.Err)
			}
		}
	}
	fmt.Fprintln(w, "\nNEXT STEPS:")
	fmt.Fprintln(w, "  1. Inspect: docker compose -f "+s.Compose+" ps")
	fmt.Fprintln(w, "  2. Logs:    docker compose -f "+s.Compose+" logs --tail=50")
	fmt.Fprintln(w, "  3. Retry:   orvyn apply (idempotent where possible)")
	fmt.Fprintln(w, "  4. If images missing due to network, pull or fix connectivity then retry")
	fmt.Fprintln(w, "\nDo NOT delete files blindly. Review above before making changes.")
}

// HasPartial reports whether a previous apply was interrupted. State that
// exists but cannot be read or parsed is treated as partial (fail-closed): a
// corrupted journal means an earlier run may have stopped mid-way, so the next
// run must surface it instead of silently starting over.
func HasPartial() bool {
	s, err := Load()
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return true
	}
	return s.Status == "in_progress"
}
