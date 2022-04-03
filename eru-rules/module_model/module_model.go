package module_model

type ModuleProjectI interface {
}

type Project struct {
	ProjectId string `eru:"required"`
	DMNs      map[string]DMN
}

type DMN struct {
	DmnName string `eru:"required"`
}
