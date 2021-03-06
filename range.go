// Copyright 2014 The Semver Package Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"bytes"
)

// Range is a subset of the universe of Versions: It can have a lower and upper boundary.
// For example, "1.2–2.0" is such a Range, with two boundaries.
type Range struct {
	lower       Version
	upper       Version
	hasLower    bool
	equalsLower bool
	hasUpper    bool
	equalsUpper bool
}

// NewRange translates into a Range.
func NewRange(str []byte) (Range, error) {
	if len(str) == 0 || (len(str) == 1 && (str[0] == '*' || str[0] == 'x')) {
		// An empty Range contains everything.
		return Range{}, nil
	}
	isNaturalRange := true
	if bytes.HasSuffix(str, []byte(".x")) || bytes.HasSuffix(str, []byte(".*")) {
		str = bytes.TrimRight(str, ".x*")
		isNaturalRange = false
	}
	if str[0] == '^' || str[0] == '~' {
		return newRangeByShortcut(str)
	}

	var upperBound, lowerBound bool = true, true
	if len(str) >= 2 {
		lowerBound = !(str[0] == '<' || str[1] == '<')
		upperBound = !(str[0] == '>' || str[1] == '>')
	}
	var leftEnd, rightStart int
	if idx := bytes.IndexByte(str, byte(' ')); idx > 1 {
		leftEnd = idx
	} else if idx = bytes.IndexByte(str, byte(',')); idx > 1 {
		leftEnd = idx
	} else {
		leftEnd = len(str)
		rightStart = leftEnd
	}
	if rightStart == 0 {
		rightStart = bytes.LastIndexByte(str, byte(' ')) + 1
		if rightStart <= 0 {
			rightStart = bytes.LastIndexByte(str, byte(',')) + 1
		}
	}

	isNaturalRange = isNaturalRange && leftEnd != rightStart && (len(str)-rightStart) > 0
	if !isNaturalRange {
		leftDotCount := bytes.Count(str[:leftEnd], []byte{'.'})
		switch leftDotCount {
		case 1:
			return newRangeByShortcut(append([]byte{'~'}, str...))
		case 0:
			return newRangeByShortcut(append([]byte{'^'}, str...))
		}
	}
	vr := Range{}
	if leftEnd == rightStart {
		err := vr.setBound(str, lowerBound, upperBound)
		return vr, err
	}

	if err := vr.setBound(str[:leftEnd], true, false); err != nil {
		return vr, err
	}
	if err := vr.setBound(str[rightStart:], false, true); err != nil {
		return vr, err
	}

	return vr, nil
}

func (r *Range) setBound(str []byte, isLower, isUpper bool) error {
	var versionStartIdx int
	for ; versionStartIdx < len(str); versionStartIdx++ {
		if isNumeric(str[versionStartIdx]) {
			goto startFound
		}
	}
	return errInvalidVersionString

startFound:
	var err error
	equalOk := versionStartIdx == 0 || bytes.IndexByte(str[:versionStartIdx], '=') > 0
	if isUpper {
		r.equalsUpper, r.hasUpper = equalOk, true
		err = r.upper.unmarshalText(str[versionStartIdx:])
	}
	if isLower {
		r.equalsLower, r.hasLower = equalOk, true
		if isUpper {
			r.lower = r.upper
		} else {
			err = r.lower.unmarshalText(str[versionStartIdx:])
		}
	}
	return err
}

// newRangeByShortcut covers the special case of Ranges whose boundaries
// are declared using prefixes.
func newRangeByShortcut(str []byte) (Range, error) {
	t := bytes.TrimLeft(str, "~^")
	num, err := NewVersion(t)
	if err != nil {
		return Range{}, err
	}
	if bytes.HasPrefix(t, []byte("0.0.")) {
		return NewRange(t)
	}

	r := Range{lower: num, hasLower: true, equalsLower: true, hasUpper: true, upper: Version{}}

	switch {
	case bytes.HasPrefix(t, []byte("0.")):
		r.upper.version[0] = r.lower.version[0]
		r.upper.version[1] = r.lower.version[1] + 1
	case str[0] == '^' || bytes.IndexByte(t, '.') <= -1:
		r.upper.version[0] = r.lower.version[0] + 1
	case str[0] == '~':
		r.upper.version[0] = r.lower.version[0]
		r.upper.version[1] = r.lower.version[1] + 1
	}

	return r, nil
}

// GetLowerBoundary gets you the lower (left) boundary.
func (r Range) GetLowerBoundary() *Version {
	if !r.hasLower {
		return nil
	}
	return &r.lower
}

// GetUpperBoundary gets you the high (right) boundary.
func (r Range) GetUpperBoundary() *Version {
	if !r.hasUpper {
		return nil
	}
	return &r.upper
}

// Contains returns true if a Version is inside this Range.
//
// If in doubt use IsSatisfiedBy.
func (r Range) Contains(v Version) bool {
	if r.upper == r.lower {
		return r.lower.LimitedEqual(v)
	}

	return r.satisfiesLowerBound(v) && r.satisfiesUpperBound(v)
}

// IsSatisfiedBy works like Contains,
// but rejects pre-releases if neither of the bounds is a pre-release.
//
// Use this in the context of pulling in packages because it follows the spirit of §9 SemVer.
// Also see https://github.com/npm/node-semver/issues/64
func (r Range) IsSatisfiedBy(v Version) bool {
	if !r.Contains(v) {
		return false
	}
	if v.IsAPreRelease() {
		if r.hasLower && r.lower.IsAPreRelease() && r.lower.sharesPrefixWith(v) {
			return true
		}
		if r.hasUpper && r.upper.IsAPreRelease() && r.upper.sharesPrefixWith(v) {
			return true
		}
		return false
	}
	return true
}

func (r Range) satisfiesLowerBound(v Version) bool {
	if !r.hasLower {
		return true
	}

	equal := r.lower.LimitedEqual(v)
	if r.equalsLower && equal {
		return true
	}

	return r.lower.limitedLess(v) && !equal
}

func (r Range) satisfiesUpperBound(v Version) bool {
	if !r.hasUpper {
		return true
	}

	equal := r.upper.LimitedEqual(v)
	if r.equalsUpper && equal {
		return true
	}

	if !r.equalsUpper && r.upper.version[idxReleaseType] == common {
		equal = r.upper.sharesPrefixWith(v)
	}

	return v.limitedLess(r.upper) && !equal
}

// Satisfies is a convenience function for former NodeJS developers,
// and works on two strings.
//
// Please see Range's IsSatisfiedBy for details.
func Satisfies(aVersion, aRange string) (bool, error) {
	v, err := NewVersion([]byte(aVersion))
	if err != nil {
		return false, err
	}
	r, err := NewRange([]byte(aRange))
	if err != nil {
		return false, err
	}

	return r.IsSatisfiedBy(v), nil
}
