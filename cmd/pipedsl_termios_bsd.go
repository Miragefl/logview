//go:build darwin || freebsd || netbsd

package cmd

import "golang.org/x/sys/unix"

// ioctlSetTermios:向 pty slave 写入终端属性的 ioctl 请求码(BSD 系)。
const ioctlSetTermios = unix.TIOCSETA
