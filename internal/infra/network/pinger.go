package network

import (
	"context"
	"os/exec"
)

type Pinger struct{}

func NewPinger() *Pinger {
	return &Pinger{}
}

func (p *Pinger) Ping(ctx context.Context, ip string) bool {
	// Pinga 1 vez (-c 1) e aguarda no máximo 1 segundo (-W ou -w 1 dependendo da dist linux)
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}