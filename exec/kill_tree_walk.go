package exec

import (
	"sort"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// killTreeByWalk SIGKILLs pid and every descendant discovered via gopsutil's
// Children() traversal. Runs the walk twice with a brief pause so children
// spawned during the first pass are also caught. Racy by nature — prefer
// WithProcessGroup() + the atomic pgid kill when available.
func killTreeByWalk(root int) error {
	for pass := 0; pass < 2; pass++ {
		nodes := collectDescendants(int32(root))
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
	return nil
}

type treeNode struct {
	pid   int32
	depth int
}

func collectDescendants(root int32) []treeNode {
	proc, err := process.NewProcess(root)
	if err != nil {
		return nil
	}
	out := []treeNode{{pid: root, depth: 0}}
	var walk func(p *process.Process, depth int)
	walk = func(p *process.Process, depth int) {
		kids, err := p.Children()
		if err != nil {
			return
		}
		for _, k := range kids {
			out = append(out, treeNode{pid: k.Pid, depth: depth + 1})
			walk(k, depth+1)
		}
	}
	walk(proc, 0)
	return out
}
