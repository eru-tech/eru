package module_store

import (
	"github.com/eru-tech/eru/eru-store/store"
)

type StoreHolder struct {
	Store ModuleStoreI
}

type ModuleStoreI interface {
	store.StoreI
}

type ModuleStore struct {
	store.FileStore
}
