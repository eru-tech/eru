package routes

import (
	"log"
	"net/http"
	"sync"
)

type Job struct {
	id       int
	request  *http.Request
	url      string
	vars     *TemplateVars
	async    bool
	asyncMsg string
}
type Result struct {
	job          Job
	response     *http.Response
	responseVars *TemplateVars
	responseErr  error
}
type FuncJob struct {
	id            int
	request       *http.Request
	funcStep      *FuncStep
	reqVars       map[string]*TemplateVars
	resVars       map[string]*TemplateVars
	asyncMessage  string
	mainRouteName string
	funcThread    int
	loopThread    int
}

type FuncResult struct {
	job          FuncJob
	response     *http.Response
	responseVars *TemplateVars
	responseErr  error
}

func worker(route *Route, wg *sync.WaitGroup, jobs chan Job, results chan Result) {
	//var currentJob Job
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued worker: ", r)
			//output := Result{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//results <- output
			wg.Done()
		}
	}()
	for job := range jobs {
		//currentJob = job
		resp, r, e := route.RunRoute(job.request, job.url, job.vars, job.async, job.asyncMsg)
		output := Result{job, resp, r, e}
		results <- output
	}
	wg.Done()
}

func createWorkerPool(route *Route, noOfWorkers int, jobs chan Job, results chan Result) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued createWorkerPool: ", r)
			return
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go worker(route, &wg, jobs, results)
	}
	wg.Wait()
	close(results)
}

func allocate(req *http.Request, u string, vars *TemplateVars, loopArray []interface{}, jobs chan Job, async bool, asyncMsg string) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued allocate: ", r)
		}
	}()
	loopCounter := 0
	log.Print("printing loopArray from allocate = ", loopArray)
	log.Print(len(loopArray))
	for loopCounter < len(loopArray) {
		laVars := *vars
		laVars.LoopVar = loopArray[loopCounter]
		log.Print(laVars.LoopVars)
		log.Print(laVars.LoopVar)
		job := Job{loopCounter, req, u, &laVars, async, asyncMsg}
		jobs <- job
		loopCounter++
	}
	close(jobs)
}

func allocateFunc(req *http.Request, funcSteps map[string]*FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, funcJobs chan FuncJob, mainRouteName string, funcThread int, loopThread int) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued allocateFunc: ", r)
		}
	}()
	loopCounter := 0
	for _, fs := range funcSteps {
		funcJob := FuncJob{loopCounter, req, fs, reqVars, resVars, "", mainRouteName, funcThread, loopThread}
		funcJobs <- funcJob
		loopCounter++
	}
	log.Print("len(funcJobs) allocateFunc= ", len(funcJobs))
	close(funcJobs)
}
func createWorkerPoolFunc(noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued createWorkerPoolFunc: ", r)
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go workerFunc(&wg, funcJobs, funcResults)
	}
	wg.Wait()
	close(funcResults)
}
func workerFunc(wg *sync.WaitGroup, funcJobs chan FuncJob, funcResults chan FuncResult) {
	//var currentJob FuncJob
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued workerFunc: ", r)
			//output := FuncResult{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//funcResults <- output
			wg.Done()
		}
	}()
	for funcJob := range funcJobs {
		//currentJob = funcJob
		if funcJob.mainRouteName == "" {
			funcJob.mainRouteName = funcJob.funcStep.GetRouteName()
		}
		resp, e := funcJob.funcStep.RunFuncStep(funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.mainRouteName, funcJob.funcThread, funcJob.loopThread)
		if e != nil {
			log.Print("print RunFuncStep error = ", e.Error())
		}
		//utils.PrintResponseBody(resp, fmt.Sprint("prinitng response before assiging to FuncResult in workerFunc for ",funcJob.funcStep.GetRouteName()))
		output := FuncResult{funcJob, resp, nil, e}
		funcResults <- output
	}
	wg.Done()
}

func allocateFuncInner(req *http.Request, funcStep *FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, loopArray []interface{}, asyncMessage string, funcJobs chan FuncJob, mainRouteName string, funcThread int, loopThread int) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued allocateFuncInner: ", r)
		}
	}()
	loopCounter := 0
	log.Print("len(loopArray) from allocateFuncInner", len(loopArray))
	for loopCounter < len(loopArray) {
		reqVarsI, err := cloneInterface(reqVars)
		if err != nil {
			log.Print(err)
		}
		reqVarsClone, errC := reqVarsI.(map[string]*TemplateVars)
		if !errC {
			log.Print(errC)
		}
		reqVarsClone[funcStep.GetRouteName()].LoopVar = loopArray[loopCounter]
		log.Print("reqVarsClone[funcStep.GetRouteName()].LoopVar = ", reqVarsClone[funcStep.GetRouteName()].LoopVar)
		funcJob := FuncJob{loopCounter, req, funcStep, reqVarsClone, resVars, asyncMessage, mainRouteName, funcThread, loopThread}
		funcJobs <- funcJob
		loopCounter++
	}
	close(funcJobs)
}

func createWorkerPoolFuncInner(noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued createWorkerPoolFuncInner: ", r)
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go workerFuncInner(&wg, funcJobs, funcResults)
	}
	wg.Wait()
	close(funcResults)
}
func workerFuncInner(wg *sync.WaitGroup, funcJobs chan FuncJob, funcResults chan FuncResult) {
	log.Print("inside workerFuncInner")
	log.Print("len(funcJobs) = ", len(funcJobs))
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued workerFuncInner: ", r)
			//output := FuncResult{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//funcResults <- output
			wg.Done()
		}
	}()
	for funcJob := range funcJobs {
		//currentJob = funcJob
		log.Print("funcJob.reqVars[funcJob.funcStep.GetRouteName()].LoopVar = ", funcJob.reqVars[funcJob.funcStep.GetRouteName()].LoopVar)
		if funcJob.mainRouteName == "" {
			funcJob.mainRouteName = funcJob.funcStep.GetRouteName()
		}
		resp, e := funcJob.funcStep.RunFuncStepInner(funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.mainRouteName, funcJob.asyncMessage, funcJob.funcThread, funcJob.loopThread)
		if e != nil {
			log.Print("print RunFuncStepInner error = ", e.Error())
		}
		//utils.PrintResponseBody(resp, fmt.Sprint("prinitng response before assiging to FuncResult in workerFuncInner for ",funcJob.funcStep.GetRouteName()))
		output := FuncResult{funcJob, resp, nil, e}
		funcResults <- output
	}
	wg.Done()
}

/*
func result(done chan bool, results chan Result, responses *[]http.Response, trResVars *[]TemplateVars, errs *[]error) {
	for res := range results {
		log.Println(res.job)
		log.Println(res.response)
		log.Println(res.responseErr)
		log.Println(res.responseVars)
		responses = append(&responses, *res.response)
		trResVars = append(trResVars,res.responseVars)
		errs = append(errs, &res.responseErr)
	}
	log.Print("print from result")
	log.Print("len(responses) = " , len(responses))
	log.Print("len(trVars) = " , len(trResVars))
	log.Print("len(errs) = " , len(errs))
	done <- true
}
*/
