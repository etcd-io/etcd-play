package table

import (
	"sort"
	"strconv"
)

// By returns a multiSorter that sorts using the less functions
func By(rows [][]string, lesses ...LessFunc) *MultiSorter {
	return &MultiSorter{
		data: rows,
		less: lesses,
	}
}

// LessFunc compares between two string slices.
type LessFunc func(p1, p2 *[]string) bool

func MakeAscendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		return (*row1)[idx] < (*row2)[idx]
	}
}

func MakeAscendingIntFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		num1s := (*row1)[idx]
		num1, _ := strconv.Atoi(num1s)
		num2s := (*row2)[idx]
		num2, _ := strconv.Atoi(num2s)
		return num1 < num2
	}
}

func MakeDescendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		return (*row1)[idx] > (*row2)[idx]
	}
}

func MakeDescendingIntFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		num1s := (*row1)[idx]
		num1, _ := strconv.Atoi(num1s)
		num2s := (*row2)[idx]
		num2, _ := strconv.Atoi(num2s)
		return num1 > num2
	}
}

func MakeDescendingFloat64Func(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		num1s := (*row1)[idx]
		num1, _ := strconv.ParseFloat(num1s, 64)
		num2s := (*row2)[idx]
		num2, _ := strconv.ParseFloat(num2s, 64)
		return num1 > num2
	}
}

// MultiSorter implements the Sort interface,
// sorting the two dimensional string slices within.
type MultiSorter struct {
	data [][]string
	less []LessFunc
}

// Sort sorts the rows according to LessFunc.
func (ms *MultiSorter) Sort(rows [][]string) {
	sort.Sort(ms)
}

// Len is part of sort.Interface.
func (ms *MultiSorter) Len() int {
	return len(ms.data)
}

// Swap is part of sort.Interface.
func (ms *MultiSorter) Swap(i, j int) {
	ms.data[i], ms.data[j] = ms.data[j], ms.data[i]
}

// Less is part of sort.Interface.
func (ms *MultiSorter) Less(i, j int) bool {
	p, q := &ms.data[i], &ms.data[j]
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(p, q):
			// p < q
			return true
		case less(q, p):
			// p > q
			return false
		}
		// p == q; try next comparison
	}
	return ms.less[k](p, q)
}
