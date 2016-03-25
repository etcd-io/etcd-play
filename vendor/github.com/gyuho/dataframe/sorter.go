package dataframe

import (
	"sort"
	"strconv"
	"time"
)

type (
	SortType   int
	SortOption int
)

const (
	SortType_String SortType = iota
	SortType_Number
	SortType_Duration
)

const (
	SortOption_Ascending SortOption = iota
	SortOption_Descending
)

// SortBy returns a multiSorter that sorts using the less functions
func SortBy(rows [][]string, lesses ...LessFunc) *MultiSorter {
	return &MultiSorter{
		data: rows,
		less: lesses,
	}
}

// LessFunc compares between two string slices.
type LessFunc func(p1, p2 *[]string) bool

func StringAscendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		return (*row1)[idx] < (*row2)[idx]
	}
}

func StringDescendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		return (*row1)[idx] > (*row2)[idx]
	}
}

func NumberAscendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		v1s := (*row1)[idx]
		v1, _ := strconv.ParseFloat(v1s, 64)
		v2s := (*row2)[idx]
		v2, _ := strconv.ParseFloat(v2s, 64)
		return v1 < v2
	}
}

func NumberDescendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		v1s := (*row1)[idx]
		v1, _ := strconv.ParseFloat(v1s, 64)
		v2s := (*row2)[idx]
		v2, _ := strconv.ParseFloat(v2s, 64)
		return v1 > v2
	}
}

func DurationAscendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		v1s := (*row1)[idx]
		v1, _ := time.ParseDuration(v1s)
		v2s := (*row2)[idx]
		v2, _ := time.ParseDuration(v2s)
		return v1 < v2
	}
}

func DurationDescendingFunc(idx int) func(row1, row2 *[]string) bool {
	return func(row1, row2 *[]string) bool {
		v1s := (*row1)[idx]
		v1, _ := time.ParseDuration(v1s)
		v2s := (*row2)[idx]
		v2, _ := time.ParseDuration(v2s)
		return v1 > v2
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
