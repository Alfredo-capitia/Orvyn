package apply

import (
	"fmt"
	"os"
)

func PrintRecovery(s *Session) {
	fmt.Println("\n=== PARTIAL FAILURE - RECOVERY ===")
	fmt.Printf("Session: %s (started: %s, status: %s)\n", s.ID, s.StartedAt.Format("2006-01-02 15:04:05"), s.Status)
	if len(s.Created) > 0 {
		fmt.Println("\nCreated in this session:")
		for _, p := range s.Created {
			fmt.Printf("  - %s\n", p)
		}
	}
	if len(s.Steps) > 0 {
		fmt.Println("\nSteps:")
		for _, st := range s.Steps {
			fmt.Printf("  [%s] %s %s\n", st.Status, st.Name, st.Path)
			if st.Err != "" {
				fmt.Printf("       error: %s\n", st.Err)
			}
		}
	}
	fmt.Println("\nNEXT STEPS:")
	fmt.Println("  1. Inspect: docker compose -f " + s.Compose + " ps")
	fmt.Println("  2. Logs:    docker compose -f " + s.Compose + " logs --tail=50")
	fmt.Println("  3. Retry:   orvyn apply (idempotent where possible)")
	fmt.Println("  4. If images missing due to network, pull or fix connectivity then retry")
	fmt.Println("\nDo NOT delete files blindly. Review above before making changes.")
}

func HasPartial() bool {
	s, err := Load()
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return s.Status == "in_progress"
}
