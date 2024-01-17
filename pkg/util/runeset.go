package util

import "strings"

type RuneSet map[rune]struct{}

func NewRuneSet(strings ...string) RuneSet {
	res := make(RuneSet)
	for _, s := range strings {
		res.Add(s)
	}
	return res
}
func (d RuneSet) String() string {
	sb := strings.Builder{}
	for r := range d {
		sb.WriteRune(r)
	}
	return sb.String()
}
func (d RuneSet) AddRune(r rune) {
	d[r] = struct{}{}
}
func (d RuneSet) RemoveRune(r rune) {
	delete(d, r)
}
func (d RuneSet) TestRune(r rune) (ok bool) {
	_, ok = d[r]
	return
}
func (d RuneSet) Add(s string) {
	for _, r := range s {
		d.AddRune(r)
	}
}
func (d RuneSet) Remove(s string) {
	for _, r := range s {
		d.RemoveRune(r)
	}
}
func (d RuneSet) Test(s string) bool {
	for _, r := range s {
		if !d.TestRune(r) {
			return false
		}
	}
	return true
}

var RunesLower = NewRuneSet("abcdefghijklmnopqrstuvwxyz")
var RunesUpper = NewRuneSet("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
var RunesNumeric = NewRuneSet("0123456789")
var RunesAlpha = NewRuneSet(RunesLower.String(), RunesUpper.String())
var RunesAlphanum = NewRuneSet(RunesAlpha.String(), RunesNumeric.String())
