package schema

// An example application.
type Application struct {
	Name        string `schema:"name,pk"`
	Series      string `schema:"series,find_by"`
	Subordinate bool   `schema:"subordinate"`
	// CharmURL and channel should be moved to CharmOrigin. Attempting it should
	// be relatively straight forward, but very time consuming.
	// When moving to CharmHub or removing CharmStore from Juju it should be
	// tackled then.
	//CharmURL *charm.URL `schema:"charmurl"`
	Channel string `schema:"cs-channel"`
	//CharmOrigin *state.CharmOrigin `schema:"charm-origin"`
	CharmModifiedVersion int  `schema:"charmmodifiedversion"`
	ForceCharm           bool `schema:"forcecharm"`
	//Life                 state.Life   `schema:"life"`
	UnitCount     int `schema:"unitcount"`
	RelationCount int `schema:"relationcount"`
	MinUnits      int `schema:"minunits"`
	//Tools                *tools.Tools `schema:"tools"`
	MetricCredentials []byte `schema:"metric-credentials"`

	// Exposed is set to true when the application is exposed.
	Exposed bool `schema:"exposed"`

	// A map for tracking the per-endpoint expose-related parameters for
	// an exposed app where keys are endpoint names or the "" value which
	// represents all application endpoints.
	ExposedEndpoints map[string]ExposedEndpoint `schema:"exposed-endpoints"`

	// CAAS related attributes.
	DesiredScale int    `schema:"scale"`
	PasswordHash string `schema:"passwordhash"`
	// Placement is the placement directive that should be used allocating units/pods.
	Placement string `schema:"placement"`
	// HasResources is set to false after an application has been removed
	// and any k8s cluster resources have been fully cleaned up.
	// Until then, the application must not be removed from the Juju model.
	HasResources bool `schema:"has-resources,find_by"`
}

type ExposedEndpoint struct {
	Spaces []string
	CIDRs  []string

	// Lorem
	Lorem string
	Ipsum string
}
