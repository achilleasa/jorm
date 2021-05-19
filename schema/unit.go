package schema

type Unit struct {
	Name        string `schema:"name,pk"`
	Application string `schema:"application,find_by"`
	Series      string `schema:"series"`
	//CharmURL               *charm.URL
	Principal              string
	Subordinates           []string
	StorageAttachmentCount int `schema:"storageattachmentcount"`
	MachineId              string
	PasswordHash           string
}
