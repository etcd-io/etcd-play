package dataframe

import (
	"strconv"
	"time"
)

// Value represents the value in data frame.
type Value interface {
	ToString() (string, bool)
	ToNumber() (float64, bool)
	ToTime(layout string) (time.Time, bool)
	ToDuration() (time.Duration, bool)
	IsNil() bool
	EqualTo(v Value) bool
}

func NewValue(v string) Value {
	return String(v)
}

type String string

func (s String) ToString() (string, bool) {
	return string(s), true
}

func (s String) ToNumber() (float64, bool) {
	f, err := strconv.ParseFloat(string(s), 64)
	return f, err == nil
}

func (s String) ToTime(layout string) (time.Time, bool) {
	t, err := time.Parse(layout, string(s))
	return t, err == nil
}

func (s String) ToDuration() (time.Duration, bool) {
	d, err := time.ParseDuration(string(s))
	return d, err == nil
}

func (s String) IsNil() bool {
	return len(s) == 0
}

func (s String) EqualTo(v Value) bool {
	tv, ok := v.(String)
	return ok && s == tv
}

type ByStringAscending []Value

func (vs ByStringAscending) Len() int {
	return len(vs)
}

func (vs ByStringAscending) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs ByStringAscending) Less(i, j int) bool {
	vs1, _ := vs[i].ToString()
	vs2, _ := vs[j].ToString()
	return vs1 < vs2
}

type ByStringDescending []Value

func (vs ByStringDescending) Len() int {
	return len(vs)
}

func (vs ByStringDescending) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs ByStringDescending) Less(i, j int) bool {
	vs1, _ := vs[i].ToString()
	vs2, _ := vs[j].ToString()
	return vs1 > vs2
}

type ByNumberAscending []Value

func (vs ByNumberAscending) Len() int {
	return len(vs)
}

func (vs ByNumberAscending) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs ByNumberAscending) Less(i, j int) bool {
	vs1, _ := vs[i].ToNumber()
	vs2, _ := vs[j].ToNumber()
	return vs1 < vs2
}

type ByNumberDescending []Value

func (vs ByNumberDescending) Len() int {
	return len(vs)
}

func (vs ByNumberDescending) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs ByNumberDescending) Less(i, j int) bool {
	vs1, _ := vs[i].ToNumber()
	vs2, _ := vs[j].ToNumber()
	return vs1 > vs2
}

type ByDurationAscending []Value

func (vs ByDurationAscending) Len() int {
	return len(vs)
}

func (vs ByDurationAscending) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs ByDurationAscending) Less(i, j int) bool {
	vs1, _ := vs[i].ToDuration()
	vs2, _ := vs[j].ToDuration()
	return vs1 < vs2
}

type ByDurationDescending []Value

func (vs ByDurationDescending) Len() int {
	return len(vs)
}

func (vs ByDurationDescending) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs ByDurationDescending) Less(i, j int) bool {
	vs1, _ := vs[i].ToDuration()
	vs2, _ := vs[j].ToDuration()
	return vs1 > vs2
}
