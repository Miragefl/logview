//go:build linux

package cmd

import "golang.org/x/sys/unix"

// ioctlSetTermios:向 pty slave 写入终端属性的 ioctl 请求码(Linux 系)。
const ioctlSetTermios = unix.TCSETS
