package hashfill

import (
	"fmt"

	geom "github.com/twpayne/go-geom"
)

// geohashBase32Alphabet is the list of charcters which make up the geohash alphabet.
var geohashBase32Alphabet = []string{
	"0", "1", "2", "3", "4", "5", "6", "7",
	"8", "9", "b", "c", "d", "e", "f", "g",
	"h", "j", "k", "m", "n", "p", "q", "r",
	"s", "t", "u", "v", "w", "x", "y", "z",
}

// FillMode is used to set how the geofence should be filled.
type FillMode int

// possible modes are intersects and contains.
const (
	FillIntersects FillMode = 1
	FillContains   FillMode = 2
)

// Filler is anything which can fill a polygon with geohashes.
type Filler interface {
	Fill(*geom.Polygon, FillMode) ([]string, error)
}

// RecursiveFiller fills the geofence by recursively searching for the largest geofence
// which is matched by the intersecting/contains predicate.
type RecursiveFiller struct {
	maxPrecision   int
	fixedPrecision bool
	container      Container
	intersector    Intersector
}

// Option allows options to be passed to RecursiveFiller
type Option func(*RecursiveFiller)

// WithMaxPrecision sets the highest precision we'll fill to.
// Defaults to 6.
func WithMaxPrecision(p int) Option {
	return func(r *RecursiveFiller) {
		r.maxPrecision = p
	}
}

// WithFixedPrecision makes the filler fill to a fixed precision rather
// than a variable one.
func WithFixedPrecision() Option {
	return func(r *RecursiveFiller) {
		r.fixedPrecision = true
	}
}

// WithPredicates overrides the default predicates used for geometric tests.
func WithPredicates(contains Container, intersects Intersector) Option {
	return func(r *RecursiveFiller) {
		r.container = contains
		r.intersector = intersects
	}
}

// NewRecursiveFiller creates a new filler with the given options.
func NewRecursiveFiller(options ...Option) *RecursiveFiller {
	filler := &RecursiveFiller{
		maxPrecision:   6,
		fixedPrecision: false,
		container:      Contains,
		intersector:    Intersects,
	}
	for _, op := range options {
		op(filler)
	}
	return filler
}

// Fill fills the polygon with geohashes.
// It works by computing a set of variable length geohashes which are contained
// in the polygon, then optionally extending those hashes out to the specified precision.
func (f RecursiveFiller) Fill(fence *geom.Polygon, mode FillMode, maxHashes int) ([]string, error) {
	hashes, err := f.computeVariableHashes(fence, mode, "", maxHashes)
	if err != nil {
		return nil, err
	}

	if !f.fixedPrecision {
		return hashes, nil
	}

	// If we want fixed precision, we have to iterate through each hash and split it down
	// to the precision we want.
	out := make([]string, 0, len(hashes))
	if len(out) > maxHashes {
		return nil, fmt.Errorf("hash limit at %d, but already have: %d", maxHashes, len(out))
	}
	for _, hash := range hashes {
		extended, err := f.extendHashToMaxPrecision(hash, maxHashes)
		if err != nil { 
			return nil, err
		}
		out = append(out, extended...)
		if len(out) > maxHashes {
			return nil, fmt.Errorf("hash limit at %d, but already have: %d", maxHashes, len(out))
		}
	}
	return out, nil
}

// extendHashToMaxPrecision recursively extends out to the max precision.
func (f RecursiveFiller) extendHashToMaxPrecision(hash string, maxHashes int) ([]string, error) {
	if len(hash) == f.maxPrecision {
		return []string{hash}, nil
	}
	hashes := make([]string, 0, 32)
	for _, next := range geohashBase32Alphabet {
		out, err := f.extendHashToMaxPrecision(hash + next, maxHashes)
		if err != nil {
			return nil, err
		}
		if len(out) > maxHashes {
			return nil, fmt.Errorf("hash limit at %d, but already have: %d", maxHashes, len(out))
		}
		hashes = append(hashes, out...)
		if len(hashes) > maxHashes {
			return nil, fmt.Errorf("hash limit at %d, but already have: %d", maxHashes, len(hashes))
		}
	}
	return hashes, nil
}

// computeVariableHashes computes the smallest list of hashes which match the geofence according to the
// fill mode.
func (f RecursiveFiller) computeVariableHashes(fence *geom.Polygon, mode FillMode, hash string, maxHashes int) ([]string, error) {
	cont, err := f.container.Contains(fence, hash)
	if err != nil {
		return nil, err
	}
	if cont {
		return []string{hash}, nil
	}

	inter, err := f.intersector.Intersects(fence, hash)
	if err != nil {
		return nil, err
	}
	if !inter {
		return nil, nil
	}

	if len(hash) == f.maxPrecision {
		// If we hit the max precision and we intersected but didn't contain,
		// it means we're at the boundary and can't go any smaller. So if we're
		// using FillIntersects, include the hash, otherwise don't.
		if mode == FillIntersects {
			return []string{hash}, nil
		}
		return nil, nil
	}

	// We didn't reach the max precision, so recurse with the next hash down.
	hashes := make([]string, 0)
	for _, next := range geohashBase32Alphabet {
		out, err := f.computeVariableHashes(fence, mode, hash+next, maxHashes)
		if err != nil {
			return nil, err
		}
		if len(out) > maxHashes {
			return nil, fmt.Errorf("hash limit at %d, but already have: %d", maxHashes, len(out))
		}
		hashes = append(hashes, out...)
		if len(hashes) > maxHashes {
			return nil, fmt.Errorf("hash limit at %d, but already have: %d", maxHashes, len(hashes))
		}
	}
	return hashes, nil
}
