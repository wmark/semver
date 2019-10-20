// Copyright 2014 The Semver Package Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !amd64

package semver

// magnitudeAwareKey is part of twoFieldKey below, and returns small figures verbatim,
// else signals magnitudes in a way susceptible to sorting.
// Parameter 'x' must be non-negative.
func magnitudeAwareKey(x int32) uint8 {
	if x <= 11 {
		return uint8(x)
	}
	// For all larger numbers, store the number of bytes +11.
	if x <= 0xffff {
		if x <= 0xff {
			return 12
		}
		return 13
	}
	if x <= 0xffffff {
		return 14
	}
	return 15
}

// twoFieldKeyGonly is part of multikeyRadixSort and derives a key from two fields in 'v'.
// The order established by the keys is ascending but not total:
// fields with great values map to a low-resolution key.
// Fields must be non-negative.
//
// This is the Go-only implementation, available for benchmarks on architectures
// that otherwise used an optimized variant.
func twoFieldKey(v *[14]int32, keyIndex uint8) uint8 {
	n1 := magnitudeAwareKey(v[keyIndex]) << 4
	if n1 >= (12 << 4) {
		return n1
	}
	return (n1 | magnitudeAwareKey(v[keyIndex+1]))
}
