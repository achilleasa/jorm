package {{basePkgName .TargetPkg}}

//
// The contents of this file have been auto-generated. Do not modify.
//

import (
	"github.com/juju/errors"
)

var (
	// ErrStoreVersionMismatch is returned by the store's ApplyChanges
	// method if any of the models that have been accessed via a
	// ModelNamespace have been concurrently updated.
	ErrStoreVersionMismatch = errors.Errorf("store contains a more recent version of the model")
)

// Store is implemented by types that can operate on the model types defined
// in the models package.
type Store interface {
	// Open initializes the store and makes it available for use.
	Open() error

	// Close the store and release any associated resources.
	Close() error

	// ModelNamespace returns a ModelNamespace instance that operations
	// can use to query models scoped to a particular Juju model UUID.
	//
	// The returned ModelNamespace instance is not safe for concurrent
	// access.
	ModelNamespace(modelUUID string) ModelNamespace

	// ApplyChanges queries the provided ModelNamespace for any changes
	// (mutations, creations and deletions) and transactionally applies
	// them to the backing store iff the model versions match the ones
	// in the backing store (i.e. the objects have not been mutated by
	// another concurrent operation).
	ApplyChanges(modelNS ModelNamespace) error
}

// ModelNamespace defines a set of model query methods provided by the backing
// store implementation whose target is limited to a particular Juju model UUID.
//
// Types that implement the ModelNamespace interface must always be treated as
// non-safe for concurrent access.
type ModelNamespace interface {
{{with nonEmbeddedModels .Models}}
{{range .}}
	{{.Name.Public}}Accessor
{{end}}
{{end}}

	// ModelUUID() returns the Juju model UUID associated with this instance.
	ModelUUID() string
}

// MutationTracker is implemented by all models.
type MutationTracker interface {
	IsModelPersisted() bool
	IsModelMutated() bool
	IsModelFieldMutated(fieldIndex int) bool
	IsModelDeleted() bool
}

