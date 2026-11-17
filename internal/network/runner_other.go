//go:build !windows

package network

import "context"

func platformRun(ctx context.Context, command Command) (string, error) {
	return "", ErrUnsupported
}
