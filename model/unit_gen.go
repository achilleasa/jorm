package model

//
// The contents of this file have been auto-generated. Do not modify.
//

type Unit struct {
	// An internal field that tracks whether the model is not yet persisted,
	// has been deleted, or if any of its fields have been mutated.
	_flags uint64

	// An internal field that tracks the version of this model.
	_version int64

	name string

	application string

	series string

	principal string

	subordinates []string

	storageAttachmentCount int

	machineId string

	passwordHash string
}

// NewUnit returns a new Unit instance with the specified version.
func NewUnit(version int64) *Unit {
	return &Unit{
		_flags:   1 << 0, // flag as not persisted.
		_version: version,
	}
}

// Mark the model as deleted.
func (m *Unit) Delete() {
	if !m.IsModelPersisted() {
		return // deleting a non-persisted model is a no-op.
	}
	m._flags |= 1 << 1 // flag as deleted.
}

func (m *Unit) GetName() string {
	return m.name
}

func (m *Unit) SetName(name string) *Unit {
	m._flags |= 1 << (2 + 0) // flag i_th field as mutated.
	m.name = name
	return m
}

func (m *Unit) GetApplication() string {
	return m.application
}

func (m *Unit) SetApplication(application string) *Unit {
	m._flags |= 1 << (2 + 1) // flag i_th field as mutated.
	m.application = application
	return m
}

func (m *Unit) GetSeries() string {
	return m.series
}

func (m *Unit) SetSeries(series string) *Unit {
	m._flags |= 1 << (2 + 2) // flag i_th field as mutated.
	m.series = series
	return m
}

func (m *Unit) GetPrincipal() string {
	return m.principal
}

func (m *Unit) SetPrincipal(principal string) *Unit {
	m._flags |= 1 << (2 + 3) // flag i_th field as mutated.
	m.principal = principal
	return m
}

func (m *Unit) GetSubordinates() []string {
	return m.subordinates
}

func (m *Unit) SetSubordinates(subordinates []string) *Unit {
	m._flags |= 1 << (2 + 4) // flag i_th field as mutated.
	m.subordinates = subordinates
	return m
}

func (m *Unit) GetStorageAttachmentCount() int {
	return m.storageAttachmentCount
}

func (m *Unit) SetStorageAttachmentCount(storageAttachmentCount int) *Unit {
	m._flags |= 1 << (2 + 5) // flag i_th field as mutated.
	m.storageAttachmentCount = storageAttachmentCount
	return m
}

func (m *Unit) GetMachineId() string {
	return m.machineId
}

func (m *Unit) SetMachineId(machineId string) *Unit {
	m._flags |= 1 << (2 + 6) // flag i_th field as mutated.
	m.machineId = machineId
	return m
}

func (m *Unit) GetPasswordHash() string {
	return m.passwordHash
}

func (m *Unit) SetPasswordHash(passwordHash string) *Unit {
	m._flags |= 1 << (2 + 7) // flag i_th field as mutated.
	m.passwordHash = passwordHash
	return m
}

// ModelVersion returns the version of the model as reported by the store.
func (m *Unit) ModelVersion() int64 { return m._version }

// IsModelPersisted returns true if the model been persisted.
func (m *Unit) IsModelPersisted() bool { return (m._flags & (1 << 0)) == 0 }

// IsModelDeleted returns true if the model has been marked for deletion.
func (m *Unit) IsModelDeleted() bool { return (m._flags & (1 << 1)) != 0 }

// IsModelMutated returns true if any of the model fields has been modified.
func (m *Unit) IsModelMutated() bool { return (m._flags & ^uint64(0b11)) != 0 }

// IsModelFieldMutated returns true if the field at fieldIndex has been modified.
func (m *Unit) IsModelFieldMutated(fieldIndex int) bool {
	return (m._flags & (1 << (2 + fieldIndex))) != 0
}

// ResetModelMutationFlags marks the model as persisted, not deleted and non-mutated.
func (m *Unit) ResetModelMutationFlags() { m._flags = 0 }
