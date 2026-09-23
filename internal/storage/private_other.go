//go:build !unix

package storage

import "os"

// Windows ACLs are not represented by os.FileMode permission bits. Path type
// and symlink checks still apply; the OS enforces access using its ACLs.
func privateStorage(os.FileInfo) bool { return true }
