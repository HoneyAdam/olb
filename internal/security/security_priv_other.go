//go:build !linux
// +build !linux

package security

// DropPrivileges is a no-op on non-Linux platforms.
func (pd *PrivilegeDropper) DropPrivileges(uid, gid int) error {
	// Privilege dropping is only supported on Linux.
	return nil
}
