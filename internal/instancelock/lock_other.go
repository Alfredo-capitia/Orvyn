//go:build !unix

package instancelock

import (
	"errors"
	"fmt"
	"os"
)

var errBusy = errors.New("instance lock is held")

func tryLock(*os.File) error {
	return fmt.Errorf("instance locking is not supported on this platform")
}

func unlock(*os.File) error { return nil }
