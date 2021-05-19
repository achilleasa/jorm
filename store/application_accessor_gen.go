package store

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"github.com/achilleasa/jorm/model"
)

// ApplicationAccessor defines an API for looking up Application instances.
type ApplicationAccessor interface {
	// Find Application by its primary key.
	FindApplicationByName(name string) (*model.Application, error)

	// Find Applications with the specified series.
	FindApplicationsBySeries(series string) ApplicationIterator
	// Find Applications with the specified hasResources.
	FindApplicationsByHasResources(hasResources bool) ApplicationIterator

	// Count the number of Applications.
	CountApplications() (int, error)

	// Iterate all Applications.
	AllApplications() ApplicationIterator

	// Create a new Application instance and queue it for insertion.
	NewApplication() *model.Application
}

// ApplicationIterator defines an API for iterating lists of Application instances.
type ApplicationIterator interface {
	// Next advances the iterator and returns false if no further data is
	// available or an error occurred. The caller must call Error() to
	// check whether the iteration was aborted due to an error.
	Next() bool

	// Application returns the current item.
	// It should only be invoked if a prior call to Next() returned true.
	// Otherwise, a nil value will be returned.
	Application() *model.Application

	// Error returns the last error (if any) encountered by the iterator.
	Error() error

	// Close the iterator and release any resources associated with it.
	Close() error
}
