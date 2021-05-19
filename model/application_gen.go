package model

//
// The contents of this file have been auto-generated. Do not modify.
//

type Application struct {
	// An internal field that tracks whether the model is not yet persisted,
	// has been deleted, or if any of its fields have been mutated.
	_flags uint64

	// An internal field that tracks the version of this model.
	_version int64

	name string

	series string

	subordinate bool

	channel string

	charmModifiedVersion int

	forceCharm bool

	unitCount int

	relationCount int

	minUnits int

	metricCredentials []byte

	exposed bool

	exposedEndpoints map[string]ExposedEndpoint

	desiredScale int

	passwordHash string

	placement string

	hasResources bool
}

// NewApplication returns a new Application instance with the specified version.
func NewApplication(version int64) *Application {
	return &Application{
		_flags:   1 << 0, // flag as not persisted.
		_version: version,
	}
}

// Mark the model as deleted.
func (m *Application) Delete() {
	if !m.IsModelPersisted() {
		return // deleting a non-persisted model is a no-op.
	}
	m._flags |= 1 << 1 // flag as deleted.
}

func (m *Application) GetName() string {
	return m.name
}

func (m *Application) SetName(name string) *Application {
	m._flags |= 1 << (2 + 0) // flag i_th field as mutated.
	m.name = name
	return m
}

func (m *Application) GetSeries() string {
	return m.series
}

func (m *Application) SetSeries(series string) *Application {
	m._flags |= 1 << (2 + 1) // flag i_th field as mutated.
	m.series = series
	return m
}

func (m *Application) IsSubordinate() bool {
	return m.subordinate
}

func (m *Application) SetSubordinate(subordinate bool) *Application {
	m._flags |= 1 << (2 + 2) // flag i_th field as mutated.
	m.subordinate = subordinate
	return m
}

func (m *Application) GetChannel() string {
	return m.channel
}

func (m *Application) SetChannel(channel string) *Application {
	m._flags |= 1 << (2 + 3) // flag i_th field as mutated.
	m.channel = channel
	return m
}

func (m *Application) GetCharmModifiedVersion() int {
	return m.charmModifiedVersion
}

func (m *Application) SetCharmModifiedVersion(charmModifiedVersion int) *Application {
	m._flags |= 1 << (2 + 4) // flag i_th field as mutated.
	m.charmModifiedVersion = charmModifiedVersion
	return m
}

func (m *Application) IsForceCharm() bool {
	return m.forceCharm
}

func (m *Application) SetForceCharm(forceCharm bool) *Application {
	m._flags |= 1 << (2 + 5) // flag i_th field as mutated.
	m.forceCharm = forceCharm
	return m
}

func (m *Application) GetUnitCount() int {
	return m.unitCount
}

func (m *Application) SetUnitCount(unitCount int) *Application {
	m._flags |= 1 << (2 + 6) // flag i_th field as mutated.
	m.unitCount = unitCount
	return m
}

func (m *Application) GetRelationCount() int {
	return m.relationCount
}

func (m *Application) SetRelationCount(relationCount int) *Application {
	m._flags |= 1 << (2 + 7) // flag i_th field as mutated.
	m.relationCount = relationCount
	return m
}

func (m *Application) GetMinUnits() int {
	return m.minUnits
}

func (m *Application) SetMinUnits(minUnits int) *Application {
	m._flags |= 1 << (2 + 8) // flag i_th field as mutated.
	m.minUnits = minUnits
	return m
}

func (m *Application) GetMetricCredentials() []byte {
	return m.metricCredentials
}

func (m *Application) SetMetricCredentials(metricCredentials []byte) *Application {
	m._flags |= 1 << (2 + 9) // flag i_th field as mutated.
	m.metricCredentials = metricCredentials
	return m
}

func (m *Application) IsExposed() bool {
	return m.exposed
}

func (m *Application) SetExposed(exposed bool) *Application {
	m._flags |= 1 << (2 + 10) // flag i_th field as mutated.
	m.exposed = exposed
	return m
}

func (m *Application) GetExposedEndpoints() map[string]ExposedEndpoint {
	return m.exposedEndpoints
}

func (m *Application) SetExposedEndpoints(exposedEndpoints map[string]ExposedEndpoint) *Application {
	m._flags |= 1 << (2 + 11) // flag i_th field as mutated.
	m.exposedEndpoints = exposedEndpoints
	return m
}

func (m *Application) GetDesiredScale() int {
	return m.desiredScale
}

func (m *Application) SetDesiredScale(desiredScale int) *Application {
	m._flags |= 1 << (2 + 12) // flag i_th field as mutated.
	m.desiredScale = desiredScale
	return m
}

func (m *Application) GetPasswordHash() string {
	return m.passwordHash
}

func (m *Application) SetPasswordHash(passwordHash string) *Application {
	m._flags |= 1 << (2 + 13) // flag i_th field as mutated.
	m.passwordHash = passwordHash
	return m
}

func (m *Application) GetPlacement() string {
	return m.placement
}

func (m *Application) SetPlacement(placement string) *Application {
	m._flags |= 1 << (2 + 14) // flag i_th field as mutated.
	m.placement = placement
	return m
}

func (m *Application) HasResources() bool {
	return m.hasResources
}

func (m *Application) SetHasResources(hasResources bool) *Application {
	m._flags |= 1 << (2 + 15) // flag i_th field as mutated.
	m.hasResources = hasResources
	return m
}

// ModelVersion returns the version of the model as reported by the store.
func (m *Application) ModelVersion() int64 { return m._version }

// IsModelPersisted returns true if the model been persisted.
func (m *Application) IsModelPersisted() bool { return (m._flags & (1 << 0)) == 0 }

// IsModelDeleted returns true if the model has been marked for deletion.
func (m *Application) IsModelDeleted() bool { return (m._flags & (1 << 1)) != 0 }

// IsModelMutated returns true if any of the model fields has been modified.
func (m *Application) IsModelMutated() bool { return (m._flags & ^uint64(0b11)) != 0 }

// IsModelFieldMutated returns true if the field at fieldIndex has been modified.
func (m *Application) IsModelFieldMutated(fieldIndex int) bool {
	return (m._flags & (1 << (2 + fieldIndex))) != 0
}

// ResetModelMutationFlags marks the model as persisted, not deleted and non-mutated.
func (m *Application) ResetModelMutationFlags() { m._flags = 0 }

type ExposedEndpoint struct {
	Spaces []string

	CIDRs []string

	Lorem string

	Ipsum string
}
