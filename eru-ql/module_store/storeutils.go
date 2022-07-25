package module_store

import (
	"errors"
	"fmt"
)

func (ms *ModuleStore) checkProjectExists(projectId string) error {
	_, ok := ms.Projects[projectId]
	if !ok {
		return errors.New(fmt.Sprint("project ", projectId, " not found"))
	}
	return nil
}

func (ms *ModuleStore) checkProjectDataSourceExists(projectId string, dbAlias string) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	_, ok := ms.Projects[projectId].DataSources[dbAlias]
	if !ok {
		return errors.New(fmt.Sprint("datasource ", dbAlias, " not found"))
	}
	return nil
}
