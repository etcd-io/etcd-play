package dataframe

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
)

// Frame contains data.
type Frame interface {
	GetHeader() []string
	AddColumn(c Column) error
	GetColumn(header string) (Column, error)
	GetColumnNumber() int
	DeleteColumn(header string) bool
	ToCSV(fpath string) error
	ToRows() ([]string, [][]string)
	Sort(header string, st SortType, so SortOption) error
}

type frame struct {
	mu       sync.Mutex
	columns  []Column
	headerTo map[string]int
}

func New() Frame {
	return &frame{
		columns:  []Column{},
		headerTo: make(map[string]int),
	}
}

func NewFromRows(header []string, rows [][]string) (Frame, error) {
	if len(rows) < 1 {
		return nil, fmt.Errorf("empty row %q", rows)
	}
	fr := New()
	headerN := len(header)
	if headerN > 0 { // use this as header
		// assume no header string at top
		cols := make([]Column, headerN)
		for i := range cols {
			cols[i] = NewColumn(header[i])
		}
		for _, row := range rows {
			rowN := len(row)
			if rowN > headerN {
				return nil, fmt.Errorf("header %q is not specified correctly for %q", header, row)
			}
			for j, v := range row {
				cols[j].PushBack(NewValue(v))
			}
			if rowN < headerN { // fill in empty values
				for k := rowN; k < headerN; k++ {
					cols[k].PushBack(NewValue(""))
				}
			}
		}
		for _, c := range cols {
			if err := fr.AddColumn(c); err != nil {
				return nil, err
			}
		}
		return fr, nil
	}
	// use first row as header
	// assume header string at top
	header = rows[0]
	headerN = len(header)
	cols := make([]Column, headerN)
	for i := range cols {
		cols[i] = NewColumn(header[i])
	}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		rowN := len(row)
		if rowN > headerN {
			return nil, fmt.Errorf("header %q is not specified correctly for %q", header, row)
		}
		for j, v := range row {
			cols[j].PushBack(NewValue(v))
		}
		if rowN < headerN { // fill in empty values
			for k := rowN; k < headerN; k++ {
				cols[k].PushBack(NewValue(""))
			}
		}
	}
	for _, c := range cols {
		if err := fr.AddColumn(c); err != nil {
			return nil, err
		}
	}
	return fr, nil
}

func NewFromCSV(header []string, fpath string) (Frame, error) {
	f, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rd := csv.NewReader(f)

	// TODO: make this configurable
	rd.FieldsPerRecord = -1

	rows, err := rd.ReadAll()
	if err != nil {
		return nil, err
	}

	return NewFromRows(header, rows)
}

func (f *frame) GetHeader() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	rs := make([]string, len(f.headerTo))
	for k, v := range f.headerTo {
		rs[v] = k
	}
	return rs
}

func (f *frame) AddColumn(c Column) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	header := c.GetHeader()
	if _, ok := f.headerTo[header]; ok {
		return fmt.Errorf("%q already exists", header)
	}
	f.columns = append(f.columns, c)
	f.headerTo[header] = len(f.columns) - 1
	return nil
}

func (f *frame) GetColumn(header string) (Column, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx, ok := f.headerTo[header]
	if !ok {
		return nil, fmt.Errorf("%q does not exist", header)
	}
	return f.columns[idx], nil
}

func (f *frame) GetColumnNumber() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.columns)
}

func (f *frame) DeleteColumn(header string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx, ok := f.headerTo[header]
	if !ok {
		return false
	}
	if idx == 0 && len(f.headerTo) == 1 {
		f.headerTo = make(map[string]int)
		f.columns = []Column{}
		return true
	}

	copy(f.columns[idx:], f.columns[idx+1:])
	f.columns = f.columns[:len(f.columns)-1 : len(f.columns)-1]

	// update headerTo
	f.headerTo = make(map[string]int)
	for i, c := range f.columns {
		f.headerTo[c.GetHeader()] = i
	}
	return true
}

func (f *frame) ToRows() ([]string, [][]string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	headers := make([]string, len(f.headerTo))
	for k, v := range f.headerTo {
		headers[v] = k
	}

	var rowN int
	for _, col := range f.columns {
		n := col.Len()
		if rowN < n {
			rowN = n
		}
	}

	rows := make([][]string, rowN)
	colN := len(f.columns)
	for rowIdx := 0; rowIdx < rowN; rowIdx++ {
		row := make([]string, colN)
		for colIdx, col := range f.columns { // rowIdx * colIdx
			v, _ := col.GetValue(rowIdx)
			elem, _ := v.ToString()
			row[colIdx] = elem
		}
		rows[rowIdx] = row
	}

	return headers, rows
}

func (f *frame) ToCSV(fpath string) error {
	fi, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		fi, err = os.Create(fpath)
		if err != nil {
			return err
		}
	}
	defer fi.Close()

	wr := csv.NewWriter(fi)

	headers, rows := f.ToRows()
	if err := wr.Write(headers); err != nil {
		return err
	}
	if err := wr.WriteAll(rows); err != nil {
		return err
	}

	wr.Flush()
	return wr.Error()
}

func (f *frame) Sort(header string, st SortType, so SortOption) error {
	f.mu.Lock()
	idx, ok := f.headerTo[header]
	if !ok {
		f.mu.Unlock()
		return fmt.Errorf("%q does not exist", header)
	}
	f.mu.Unlock()

	var lesses []LessFunc
	switch st {
	case SortType_String:
		switch so {
		case SortOption_Ascending:
			lesses = []LessFunc{StringAscendingFunc(idx)}

		case SortOption_Descending:
			lesses = []LessFunc{StringDescendingFunc(idx)}
		}

	case SortType_Number:
		switch so {
		case SortOption_Ascending:
			lesses = []LessFunc{NumberAscendingFunc(idx)}

		case SortOption_Descending:
			lesses = []LessFunc{NumberDescendingFunc(idx)}
		}

	case SortType_Duration:
		switch so {
		case SortOption_Ascending:
			lesses = []LessFunc{DurationAscendingFunc(idx)}

		case SortOption_Descending:
			lesses = []LessFunc{DurationDescendingFunc(idx)}
		}
	}

	headers, rows := f.ToRows()
	// sort by idx
	SortBy(
		rows,
		lesses...,
	).Sort(rows)

	nf, err := NewFromRows(headers, rows)
	if err != nil {
		return err
	}
	v, ok := nf.(*frame)
	if !ok {
		return fmt.Errorf("cannot type assert on frame")
	}
	*f = *v
	return nil
}
