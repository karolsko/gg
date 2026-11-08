package gitx

// Layout fills the graph fields of commits, newest first. Lanes are reused
// once a parent is drawn, so the width follows how many branches are in
// flight at once rather than how many the repository has ever had.
func Layout(commits []Commit) {
	var lanes []string
	for i := range commits {
		c := &commits[i]
		col := -1
		for j, hash := range lanes {
			if hash == c.Hash {
				col = j
				c.FromAbove = true
				break
			}
		}
		if col < 0 {
			col = freeLane(&lanes)
		}
		c.Col = col
		for j, hash := range lanes {
			if hash != "" && j != col {
				c.Passing = append(c.Passing, j)
			}
		}
		lanes[col] = ""
		for n, parent := range c.Parents {
			prefer := -1
			if n == 0 {
				prefer = col
			}
			c.Downs = append(c.Downs, placeParent(&lanes, parent, prefer))
		}
	}
}

// GraphWidth is how many lanes the commits use.
func GraphWidth(commits []Commit) int {
	max := 0
	for _, c := range commits {
		if c.Col > max {
			max = c.Col
		}
		for _, col := range c.Passing {
			if col > max {
				max = col
			}
		}
		for _, col := range c.Downs {
			if col > max {
				max = col
			}
		}
	}
	if len(commits) == 0 {
		return 0
	}
	return max + 1
}

func freeLane(lanes *[]string) int {
	for i, hash := range *lanes {
		if hash == "" {
			return i
		}
	}
	*lanes = append(*lanes, "")
	return len(*lanes) - 1
}

func placeParent(lanes *[]string, hash string, prefer int) int {
	for j, have := range *lanes {
		if have == hash {
			return j
		}
	}
	if prefer >= 0 && prefer < len(*lanes) && (*lanes)[prefer] == "" {
		(*lanes)[prefer] = hash
		return prefer
	}
	dest := freeLane(lanes)
	(*lanes)[dest] = hash
	return dest
}
