//go:build linux
// +build linux

package security

import (
	"fmt"
	"syscall"
)

// DropPrivileges changes the process effective UID and GID. This is typically
// called after binding to privileged ports (< 1024) so the process continues
// running with lower privileges.
//
// The GID is set first because setgid fails once root privileges are dropped.
func (pd *PrivilegeDropper) DropPrivileges(uid, gid int) error {
	// Set supplementary groups to empty.
	if err := syscall.Setgroups([]int{gid}); err != nil {
		return fmt.Errorf("security: setgroups failed: %w", err)
	}

	// Set GID first.
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("security: setgid(%d) failed: %w", gid, err)
	}

	// Set UID.
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("security: setuid(%d) failed: %w", uid, err)
	}

	return nil
}
