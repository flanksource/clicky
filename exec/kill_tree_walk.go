package exec

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// killTreeByWalk SIGKILLs pid and every descendant discovered from a process
// table snapshot. Runs the walk twice with a brief pause so children spawned
// during the first pass are also caught. Racy by nature — prefer
// WithProcessGroup() + the atomic pgid kill when available.
func killTreeByWalk(root int) error {
	var failures []error
	for pass := 0; pass < 2; pass++ {
		nodes, err := collectDescendants(int32(root))
		if err != nil {
			failures = append(failures, err)
			nodes = []treeNode{{pid: int32(root)}}
		}
		// Kill deepest first so short-lived parents can't respawn their
		// children before we reach the leaves.
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].depth > nodes[j].depth })
		for _, n := range nodes {
			if proc, err := process.NewProcess(n.pid); err == nil {
				_ = proc.Kill()
			}
		}
		if pass == 0 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return errors.Join(failures...)
}

type treeNode struct {
	pid   int32
	depth int
}

func collectDescendants(root int32) ([]treeNode, error) {
	pids, err := process.Pids()
	if err != nil {
		return nil, fmt.Errorf("enumerate descendants of %d: %w", root, err)
	}
	children := make(map[int32][]int32)
	for _, pid := range pids {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		parent, err := proc.Ppid()
		if err == nil {
			children[parent] = append(children[parent], pid)
		}
	}

	out := []treeNode{{pid: root, depth: 0}}
	seen := map[int32]bool{root: true}
	for i := 0; i < len(out); i++ {
		for _, child := range children[out[i].pid] {
			if seen[child] {
				continue
			}
			seen[child] = true
			out = append(out, treeNode{pid: child, depth: out[i].depth + 1})
		}
	}
	return out, nil
}
