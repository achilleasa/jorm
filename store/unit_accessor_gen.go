package store

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"github.com/achilleasa/jorm/model"
)

// UnitAccessor defines an API for looking up Unit instances.
type UnitAccessor interface {
	// Find Unit by its primary key.
	FindUnitByName(name string) (*model.Unit, error)

	// Find Units with the specified application.
	FindUnitsByApplication(application string) UnitIterator

	// Count the number of Units.
	CountUnits() (int, error)

	// Iterate all Units.
	AllUnits() UnitIterator

	// Create a new Unit instance and queue it for insertion.
	NewUnit() *model.Unit
}

// UnitIterator defines an API for iterating lists of Unit instances.
type UnitIterator interface {
	// Next advances the iterator and returns false if no further data is
	// available or an error occurred. The caller must call Error() to
	// check whether the iteration was aborted due to an error.
	Next() bool

	// Unit returns the current item.
	// It should only be invoked if a prior call to Next() returned true.
	// Otherwise, a nil value will be returned.
	Unit() *model.Unit

	// Error returns the last error (if any) encountered by the iterator.
	Error() error

	// Close the iterator and release any resources associated with it.
	Close() error
}
