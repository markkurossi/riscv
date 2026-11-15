
# Dropping Privileges

``` go
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func dropPrivileges(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}

	// Set supplementary groups for the target user.
	groupIDs, err := u.GroupIds()
	if err != nil {
		return err
	}

	groups := make([]int, len(groupIDs))
	for i, g := range groupIDs {
		groups[i], err = strconv.Atoi(g)
		if err != nil {
			return err
		}
	}

	// Drop groups first.
	if err := syscall.Setgroups(groups); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}

	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid: %w", err)
	}

	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid: %w", err)
	}

	// Verify root cannot be regained.
	if err := syscall.Setuid(0); err == nil {
		return fmt.Errorf("privilege drop failed")
	}

	return nil
}

func main() {
	if err := dropPrivileges("nobody"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("uid=%d gid=%d\n", os.Getuid(), os.Getgid())
}
```
