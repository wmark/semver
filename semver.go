// Copyright 2014 The Semver Package Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"bytes"
)

// Errors that are thrown during parsing.
const (
	errInvalidVersionString InvalidStringValue = "Given string does not resemble a Version"
	errTooManyColumns       InvalidStringValue = "Version consists of too many columns"
	errVersionStringLength  InvalidStringValue = "Version is too long"
	errInvalidBuildSuffix   InvalidStringValue = "Version has a '+' but no +buildNNN suffix"
	errInvalidType          InvalidStringValue = "Cannot read this type into a Version"
	errOutOfBounds          InvalidStringValue = "The source representation does not fit into a Version"
)

// alpha = -4, beta = -3, pre = -2, rc = -1, common = 0, revision = 1, patch = 2
const (
	alpha = iota - 4
	beta
	pre
	rc
	common
	revision
	patch
)

const (
	idxReleaseType   = 4
	idxRelease       = 5
	idxSpecifierType = 9
	idxSpecifier     = 10
)

var releaseDesc = map[int]string{
	alpha:    "alpha",
	beta:     "beta",
	pre:      "pre",
	rc:       "rc",
	revision: "r",
	patch:    "p",
}

var releaseValue = map[string]int{
	"alpha": alpha,
	"beta":  beta,
	"pre":   pre,
	"":      pre,
	"rc":    rc,
	"r":     revision,
	"p":     patch,
}

var buildsuffix = []byte("+build")

// InvalidStringValue instances are returned as error on any conversion failure.
type InvalidStringValue string

// Error implements the error interface.
func (e InvalidStringValue) Error() string { return string(e) }

// IsInvalid satisfies a function IsInvalid().
// This is used by some input validator packages.
func (e InvalidStringValue) IsInvalid() bool { return true }

// Version represents a version:
// Columns consisting of up to four unsigned integers (1.2.4.99)
// optionally further divided into 'release' and 'specifier' (1.2-634.0-99.8).
type Version struct {
	// 0–3: version, 4: releaseType, 5–8: releaseVer, 9: releaseSpecifier, 10–: specifier
	version [14]int32
	build   int32
	_       int32
}

// MustParse is NewVersion for strings, and panics on errors.
//
// Use this in tests or with constants, e. g. whenever you control the input.
//
// This is a convenience function for a cloud plattform provider.
func MustParse(str string) Version {
	ver, err := NewVersion([]byte(str))
	if err != nil {
		panic(err.Error())
	}
	return ver
}

// NewVersion translates the given string, which must be free of whitespace,
// into a single Version.
//
// An io.Reader will give you []byte, hence this (and most functions internally)
// works on []byte to have as few conversion as possible.
func NewVersion(str []byte) (Version, error) {
	ver := Version{}
	err := (&ver).unmarshalText(str)
	return ver, err
}

// Parse reads a string into the given version, overwriting any existing values.
//
// Deprecated: Use the idiomatic UnmarshalText instead.
func (t *Version) Parse(str string) error {
	t.version = [14]int32{}
	t.build = 0

	return t.unmarshalText([]byte(str))
}

func isNumeric(ch byte) bool {
	return ((ch - '0') <= 9)
}

func isSmallLetter(ch byte) bool {
	// case insensitive: (ch | 0x20)
	return ((ch - 'a') <= ('z' - 'a'))
}

// atoui consumes up to n byte from b to convert them into |val|.
func atoui(b []byte) (n int, val uint32) {
	for ; n <= 10 && n < len(b); n++ {
		v := b[n] - '0' // see above 'isNumeric'
		if v > 9 {
			break
		}
		val = val*10 + uint32(v)
	}
	return
}

// unmarshalText implements the encoding.TextUnmarshaler interface,
// but assumes the data structure is pristine.
func (t *Version) unmarshalText(str []byte) error {
	var idx, fieldNum, column int
	var strlen = len(str)

	if strlen > 1 && str[idx] == 'v' {
		idx++
	}

	for idx < strlen {
		r := str[idx]
		switch {
		case r == '.':
			idx++
			column++
			if column >= 4 || idx >= strlen {
				return errTooManyColumns
			}
			fieldNum++
			fallthrough
		case isNumeric(r):
			idxDelta, n := atoui(str[idx:])
			if idxDelta == 0 || idxDelta >= 10 { // strlen(maxInt) is 10
				return errInvalidVersionString
			}
			t.version[fieldNum] = int32(n)

			idx += idxDelta
		case r == '-' || r == '_':
			idx++
			if idx < strlen && isNumeric(str[idx]) {
				column = 0
				switch {
				case fieldNum < idxReleaseType:
					fieldNum = idxReleaseType + 1
				case fieldNum < idxSpecifierType:
					fieldNum = idxSpecifierType + 1
				default:
					return errInvalidVersionString
				}
				continue
			}
			fallthrough
		case isSmallLetter(r):
			toIdx := idx + 1
			for ; toIdx < strlen && isSmallLetter(str[toIdx]); toIdx++ {
			}

			if toIdx > strlen {
				return errInvalidVersionString
			}
			typ, known := releaseValue[string(str[idx:toIdx])]
			if !known {
				return errInvalidVersionString
			}
			switch {
			case fieldNum < idxReleaseType:
				fieldNum = idxReleaseType
			case fieldNum < idxSpecifierType:
				fieldNum = idxSpecifierType
			default:
				return errInvalidVersionString
			}
			t.version[fieldNum] = int32(typ)
			if toIdx+1 < strlen && str[toIdx] == '.' {
				toIdx++
			}

			fieldNum++
			column = 0
			idx = toIdx
		case r == '+':
			if strlen < idx+len(buildsuffix)+1 || !bytes.Equal(str[idx:idx+len(buildsuffix)], buildsuffix) {
				return errInvalidBuildSuffix
			}
			idx += len(buildsuffix)
			idxDelta, n := atoui(str[idx:])
			if idxDelta > 9 || idx+idxDelta < strlen {
				return errInvalidBuildSuffix
			}
			t.build = int32(n)
			return nil
		default:
			return errInvalidVersionString
		}
	}

	return nil
}

// signDelta returns the signum of the difference,
// whose precision can be limited by 'cuttofIdx'.
func signDelta(a, b [14]int32, cutoffIdx int) int8 {
	_ = a[0:cutoffIdx]
	for i := 0; i < len(a) && i < cutoffIdx; i++ {
		if a[i] == b[i] {
			continue
		}
		x := a[i] - b[i]
		return int8((x >> 31) - (-x >> 31))
	}
	return 0
}

// limitedLess compares two Versions
// with a precision limited to version, (pre-)release type and (pre-)release version.
//
// Commutative.
func (t Version) limitedLess(o Version) bool {
	return signDelta(t.version, o.version, idxSpecifierType) < 0
}

// LimitedEqual returns true if two versions share the same: prefix,
// which is the "actual version", (pre-)release type, and (pre-)release version.
// The exception are patch-levels, which are always equal.
//
// Use this, for example, to tell a beta from a regular version;
// or to accept a patched version as regular version.
//
// A thing confusing but convention is to read this from right to left.
func (t Version) LimitedEqual(o Version) bool {
	if t.version[idxReleaseType] == common && o.version[idxReleaseType] > common {
		return t.sharesPrefixWith(o)
	}
	return signDelta(t.version, o.version, idxSpecifierType) == 0
}

// IsAPreRelease is used to discriminate pre-releases.
func (t Version) IsAPreRelease() bool {
	return t.version[idxReleaseType] < common
}

// sharesPrefixWith compares two Versions with a fixed limited precision.
//
// A 'prefix' is the major, minor, patch and revision number.
// For example: 1.2.3.4…
func (t Version) sharesPrefixWith(o Version) bool {
	return signDelta(t.version, o.version, idxReleaseType) == 0
}

// Major returns the major of a version.
func (t Version) Major() int {
	return int(t.version[0])
}

// Minor returns the minor of a version.
func (t Version) Minor() int {
	return int(t.version[1])
}

// Patch returns the patch of a version.
func (t Version) Patch() int {
	return int(t.version[2])
}

// VersionPtrs represents an array with elements derived from~ but smaller than Versions.
// Use this a proxy for sorting of large collections of Versions,
// to minimize memory moves.
type VersionPtrs []*Version

var _ interface {
	Sort()
	// These are from sort.Interface:
	Len() int
	Less(int, int) bool
	Swap(int, int)
} = VersionPtrs{}

// Len implements the sort.Interface.
func (p VersionPtrs) Len() int {
	return len(p)
}

// Swap implements the sort.Interface.
func (p VersionPtrs) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
