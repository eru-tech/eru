package module_model

type ModuleProjectI interface {
}

type Project struct {
	ProjectId string `eru:"required"`
}
