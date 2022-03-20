package store

type StoreI interface {
	LoadStore(fp string, ms StoreI) (err error)
	GetStoreByteArray(fp string) (b []byte, err error)
	SaveStore(fp string, ms StoreI) (err error)
	SetDbType(dbtype string)

	//SaveProject(projectId string, realStore StoreI) error
	//RemoveProject(projectId string, realStore StoreI) error
	//GetProjectConfig(projectId string) (*model.ProjectI, error)
	//GetProjectList() []map[string]interface{}
}

type Store struct {
	//Projects map[string]*model.Project //ProjectId is the key
}

func (store *Store) SetDbType(dbtype string) {
	//do nothing
}

/*
func (store *Store) SaveProject(projectId string, realStore StoreI) error {
	//TODO to handle edit project once new project attributes are finalized
	if _, ok := store.Projects[projectId]; !ok {
		project := new(model.Project)
		project.ProjectId = projectId
		if store.Projects == nil {
			store.Projects = make(map[string]*model.Project)
		}
		store.Projects[projectId] = project
		log.Print("SaveStore called from SaveProject")
		return realStore.SaveStore("")
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " already exists"))
	}
}

func (store *Store) RemoveProject(projectId string, realStore StoreI) error {
	if _, ok := store.Projects[projectId]; ok {
		delete(store.Projects, projectId)
		log.Print("SaveStore called from RemoveProject")
		return realStore.SaveStore("")
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (store *Store) GetProjectConfig(projectId string) (*model.ProjectI, error) {
	log.Println("inside GetProjectConfig")
	if _, ok := store.Projects[projectId]; ok {
		//log.Println(store.Projects[projectId])
		var p model.ProjectI
		p = store.Projects[projectId]
		return &p, nil
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (store *Store) GetProjectList() []map[string]interface{} {
	projects := make([]map[string]interface{}, len(store.Projects))
	i := 0
	for k := range store.Projects {
		project := make(map[string]interface{})
		project["projectName"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}
*/
