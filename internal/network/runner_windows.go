package network

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

type limitedBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	size := len(data)
	remaining := 1024*1024 - len(buffer.data)
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		buffer.data = append(buffer.data, data...)
	}
	return size, nil
}

func platformRun(ctx context.Context, command Command) (string, error) {
	systemRoot := os.Getenv("SystemRoot")
	if !filepath.IsAbs(systemRoot) {
		return "", errors.New("无法确定 Windows 系统目录")
	}
	var executable string
	switch command.Program {
	case "powershell.exe":
		executable = filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	case "netsh.exe":
		executable = filepath.Join(systemRoot, "System32", "netsh.exe")
	default:
		return "", errors.New("不允许执行此程序")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, executable, command.Args...)
	process.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	process.WaitDelay = 2 * time.Second
	var output limitedBuffer
	process.Stdout = &output
	process.Stderr = &output
	err := process.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	text := string(output.data)
	if command.Program == "netsh.exe" {
		text = decodeOEM(output.data)
	}
	return text, err
}

func decodeOEM(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	convert := syscall.NewLazyDLL("kernel32.dll").NewProc("MultiByteToWideChar")
	size, _, _ := convert.Call(1, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0, 0)
	if size == 0 {
		return string(data)
	}
	wide := make([]uint16, int(size))
	written, _, _ := convert.Call(1, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&wide[0])), size)
	if written == 0 {
		return string(data)
	}
	return string(utf16.Decode(wide[:int(written)]))
}
