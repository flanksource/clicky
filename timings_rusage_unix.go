//go:build unix

package clicky

import (
	"syscall"

	rpchttp "github.com/flanksource/clicky/rpc/http"
	"github.com/flanksource/commons/logger"
)

func processUsage() ProcessUsage {
	return ProcessUsage{Self: rusage(syscall.RUSAGE_SELF), Children: rusage(syscall.RUSAGE_CHILDREN)}
}

func rusage(who int) ResourceUsage {
	var usage syscall.Rusage
	if err := syscall.Getrusage(who, &usage); err != nil {
		logger.Warnf("getrusage(%d): %v", who, err)
		return ResourceUsage{}
	}
	return ResourceUsage{
		UserUs:      usage.Utime.Nano() / 1000,
		SysUs:       usage.Stime.Nano() / 1000,
		MaxRSSBytes: rpchttp.MaxRSSBytes(int64(usage.Maxrss)),
		InBlocks:    int64(usage.Inblock),
		OutBlocks:   int64(usage.Oublock),
	}
}
