package schemaform

import "sort"

// splitRow is what the Advanced split knows about one shown field.
type splitRow struct {
	required      bool
	hasValue      bool
	discriminator bool
	// positional is the command's positional argument (x-clicky role
	// "positional"): its main input, shown even when optional and empty.
	positional bool
}

func splitRowFor(f *field) splitRow {
	return splitRow{
		required:      required(f),
		hasValue:      !isEmpty(f.value()),
		discriminator: f.discriminator,
		positional:    f.base.Clicky != nil && f.base.Clicky.Role == "positional",
	}
}

func (r splitRow) pinned() bool { return r.required || r.discriminator || r.positional }

// score ranks a row the way clicky-ui's orderByPriority does, with the
// discriminator (its value picks the branch) and then the positional
// argument ahead of everything.
func (r splitRow) score() int {
	score := 0
	if r.discriminator {
		score += 8
	}
	if r.positional {
		score += 4
	}
	if r.required {
		score += 2
	}
	if r.hasValue {
		score++
	}
	return score
}

// splitAdvanced chooses the rows the form shows up front and the rows it
// folds under Advanced. The discriminator and required rows are always shown,
// even past budget; the other rows fill what is left of budget, those with a
// value first. Shown rows come back in priority order, folded rows in
// document order; ties keep document order.
func splitAdvanced(rows []splitRow, budget int) (primary, advanced []int) {
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return rows[order[a]].score() > rows[order[b]].score() })
	room := budget
	for _, r := range rows {
		if r.pinned() {
			room--
		}
	}
	shown := make([]bool, len(rows))
	for _, i := range order {
		switch {
		case rows[i].pinned():
			shown[i] = true
		case room > 0:
			shown[i] = true
			room--
		}
		if shown[i] {
			primary = append(primary, i)
		}
	}
	for i := range rows {
		if !shown[i] {
			advanced = append(advanced, i)
		}
	}
	return primary, advanced
}
