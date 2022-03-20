package main

import (
	"github.com/eru-tech/eru/eru-files/file_server"
	"github.com/eru-tech/eru/eru-files/module_store"
	"github.com/eru-tech/eru/eru-server/server"
	"log"
	"os"
)

var port = "8081"

/*
type parentI interface{
	t1()
}

type childI interface{
	parentI
	t2()
}

type parentS struct {
	parentId string
}

type childS struct {
	parentS
	childId string
}

func (p parentS) t1() {
	log.Print("t1 called for parentS")
}

func (c childS) t1() {
	log.Print("t1 called for childS")
}

func (c childS) t2() {
	log.Print("t2 called for childS")
}
*/

func main() {
	/*var p parentI
	p = parentS{}
	p.t1()

	var c childI
	c = childS{}
	c.t1()
	c.t2()
	*/
	log.Println("inside main of eru-files")
	envPort := os.Getenv("ERUFILESPORT")
	if envPort != "" {
		port = envPort
	}
	store, _ := file_server.StartUp()
	sh := new(module_store.StoreHolder)
	sh.Store = store
	sr, _, e := server.Init()
	file_server.AddFileRoutes(sr, sh)
	if e != nil {
		log.Print(e)
	}
	server.Launch(sr, port)
}
