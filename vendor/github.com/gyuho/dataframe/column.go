package dataframe

import (
	"fmt"
	"sort"
	"sync"
)

// Column represents column-based data.
type Column interface {
	// RowNumber returns the number of rows of the Column.
	RowNumber() int

	// GetHeader returns the header of the Column.
	GetHeader() string

	// UpdateHeader updates the header of the Column.
	UpdateHeader(header string)

	// GetValue returns the Value in the row. It returns error if the row
	// is out of index range.
	GetValue(row int) (Value, error)

	// SetValue overwrites the value
	SetValue(row int, v Value) error

	// FindValue finds the first Value, and returns the row number.
	// It returns -1 and false if the value does not exist.
	FindValue(v Value) (int, bool)

	// FindLastValue finds the last Value, and returns the row number.
	// It returns -1 and false if the value does not exist.
	FindLastValue(v Value) (int, bool)

	// Front returns the first row Value.
	Front() (Value, bool)

	// FrontNonNil returns the first non-nil Value from the first row.
	FrontNonNil() (Value, bool)

	// Back returns the last row Value.
	Back() (Value, bool)

	// BackNonNil returns the first non-nil Value from the last row.
	BackNonNil() (Value, bool)

	// PushFront adds a Value to the front of the Column.
	PushFront(v Value) int

	// PushBack appends the Value to the Column.
	PushBack(v Value) int

	// DeleteRow deletes a row by index.
	DeleteRow(row int) (Value, error)

	// DeleteRows deletes rows by index [start, end).
	DeleteRows(start, end int) error

	// KeepRows keeps the rows by index [start, end).
	KeepRows(start, end int) error

	// PopFront deletes the value at front.
	PopFront() (Value, bool)

	// PopBack deletes the last value.
	PopBack() (Value, bool)

	// Appends adds the Value to the Column until it reaches the target size.
	Appends(v Value, targetSize int) error

	// SortByStringAscending sorts Column in string ascending order.
	SortByStringAscending()

	// SortByStringDescending sorts Column in string descending order.
	SortByStringDescending()

	// SortByNumberAscending sorts Column in number(float) ascending order.
	SortByNumberAscending()

	// SortByNumberDescending sorts Column in number(float) descending order.
	SortByNumberDescending()

	// SortByDurationAscending sorts Column in time.Duration ascending order.
	SortByDurationAscending()

	// SortByDurationDescending sorts Column in time.Duration descending order.
	SortByDurationDescending()
}

type column struct {
	mu     sync.Mutex
	header string
	size   int
	data   []Value
}

func NewColumn(hd string) Column {
	return &column{
		header: hd,
		size:   0,
		data:   []Value{},
	}
}

func (c *column) RowNumber() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.size
}

func (c *column) GetHeader() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.header
}

func (c *column) UpdateHeader(header string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.header = header
}

func (c *column) GetValue(row int) (Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if row > c.size-1 {
		return nil, fmt.Errorf("index out of range (got %d for size %d)", row, c.size)
	}
	return c.data[row], nil
}

func (c *column) SetValue(row int, v Value) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if row > c.size-1 {
		return fmt.Errorf("index out of range (got %d for size %d)", row, c.size)
	}
	c.data[row] = v
	return nil
}

func (c *column) FindValue(v Value) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.data {
		if c.data[i].EqualTo(v) {
			return i, true
		}
	}
	return -1, false
}

func (c *column) FindLastValue(v Value) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var idx int
	for i := range c.data {
		if c.data[i].EqualTo(v) {
			idx = i
		}
	}
	if idx != 0 {
		return idx, true
	}
	return -1, false
}

func (c *column) Front() (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size == 0 {
		return nil, false
	}
	v := c.data[0]
	return v, true
}

func (c *column) FrontNonNil() (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size == 0 {
		return nil, false
	}
	for _, v := range c.data {
		if !v.IsNil() {
			return v, true
		}
	}
	return nil, false
}

func (c *column) Back() (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size == 0 {
		return nil, false
	}
	v := c.data[c.size-1]
	return v, true
}

func (c *column) BackNonNil() (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size == 0 {
		return nil, false
	}
	for i := c.size - 1; i > 0; i-- {
		v := c.data[i]
		if !v.IsNil() {
			return v, true
		}
	}
	return nil, false
}

func (c *column) PushFront(v Value) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	temp := make([]Value, c.size+1)
	temp[0] = v
	copy(temp[1:], c.data)
	c.data = temp
	c.size++
	return c.size
}

func (c *column) PushBack(v Value) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = append(c.data, v)
	c.size++
	return c.size
}

func (c *column) DeleteRow(row int) (Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if row > c.size-1 {
		return nil, fmt.Errorf("index out of range (got %d for size %d)", row, c.size)
	}
	v := c.data[row]
	copy(c.data[row:], c.data[row+1:])
	c.data = c.data[:len(c.data)-1 : len(c.data)-1]
	c.size--
	return v, nil
}

func (c *column) DeleteRows(start, end int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if start < 0 || end < 0 || start > end {
		return fmt.Errorf("wrong range %d %d", start, end)
	}
	if start > c.size {
		return fmt.Errorf("index out of range (start %d, size %d)", start, c.size)
	}
	if end > c.size {
		return fmt.Errorf("index out of range (end %d, size %d)", end, c.size)
	}
	if start == end {
		return nil
	}

	delta := end - start
	c.size = c.size - delta
	var nds []Value
	for i := range c.data {
		if i >= start && i < end {
			continue
		}
		nds = append(nds, c.data[i])
	}
	c.data = nds
	return nil
}

func (c *column) KeepRows(start, end int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if start < 0 || end < 0 || start > end {
		return fmt.Errorf("wrong range %d %d", start, end)
	}
	if start > c.size {
		return fmt.Errorf("index out of range (start %d, size %d)", start, c.size)
	}
	if end > c.size {
		return fmt.Errorf("index out of range (end %d, size %d)", end, c.size)
	}
	if start == end {
		return nil
	}

	delta := end - start
	c.size = delta
	var nds []Value
	for _, v := range c.data[start:end] {
		nds = append(nds, v)
	}
	c.data = nds
	return nil
}

func (c *column) PopFront() (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size == 0 {
		return nil, false
	}
	v := c.data[0]
	c.data = c.data[1:len(c.data):len(c.data)]
	c.size--
	return v, true
}

func (c *column) PopBack() (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size == 0 {
		return nil, false
	}
	v := c.data[c.size-1]
	c.data = c.data[:len(c.data)-1 : len(c.data)-1]
	c.size--
	return v, true
}

func (c *column) Appends(v Value, targetSize int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size > 0 && c.size > targetSize {
		return fmt.Errorf("cannot append with target size %d, which is less than the column size %d (can't overwrite)", targetSize, c.size)
	}

	for i := c.size; i < targetSize; i++ {
		c.data = append(c.data, v)
		c.size++
	}
	return nil
}

func (c *column) SortByStringAscending()    { sort.Sort(ByStringAscending(c.data)) }
func (c *column) SortByStringDescending()   { sort.Sort(ByStringDescending(c.data)) }
func (c *column) SortByNumberAscending()    { sort.Sort(ByNumberAscending(c.data)) }
func (c *column) SortByNumberDescending()   { sort.Sort(ByNumberDescending(c.data)) }
func (c *column) SortByDurationAscending()  { sort.Sort(ByDurationAscending(c.data)) }
func (c *column) SortByDurationDescending() { sort.Sort(ByDurationDescending(c.data)) }
