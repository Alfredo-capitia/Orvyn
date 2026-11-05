package cli

import (
	"fmt"
	"os"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/instancelock"
)

// acquireInstanceLock serialises the commands that mutate the stack. The lock
// covers the whole command, including the confirmation prompt and the Docker
// calls, so two operators (or a script and a human) cannot interleave a down
// and an apply on the same state directory.
func acquireInstanceLock(cfg *config.Config) (*instancelock.Lock, error) {
	if cfg == nil {
		return nil, fmt.Errorf("instance lock: no configuration")
	}
	if err := os.MkdirAll(cfg.StateDir, 0700); err != nil {
		return nil, fmt.Errorf("instance lock: create state dir: %w", err)
	}
	return instancelock.Acquire(instancelock.Path(cfg.StateDir))
}
