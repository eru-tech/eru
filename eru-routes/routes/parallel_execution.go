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
	id           int
	request      *http.Request
	funcStep     *FuncStep
	reqVars      map[string]*TemplateVars
	resVars      map[string]*TemplateVars
	asyncMessage string
}
type FuncResult struct {
	job          FuncJob
	response     *http.Response
	responseVars *TemplateVars
	responseErr  error
}

func worker(route *Route, wg *sync.WaitGroup, jobs chan Job, results chan Result) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued worker: ", r)
		}
	}()
	for job := range jobs {
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
	for loopCounter < len(loopArray) {
		laVars := *vars
		laVars.LoopVars = loopArray[loopCounter]
		job := Job{loopCounter, req, u, &laVars, async, asyncMsg}
		jobs <- job
		loopCounter++
	}
	close(jobs)
}

func allocateFunc(req *http.Request, funcSteps map[string]*FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, funcJobs chan FuncJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued allocateFunc: ", r)
		}
	}()
	loopCounter := 0
	for _, fs := range funcSteps {
		funcJob := FuncJob{loopCounter, req, fs, reqVars, resVars, ""}
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
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued workerFunc: ", r)
		}
	}()
	for funcJob := range funcJobs {
		resp, e := funcJob.funcStep.RunFuncStep(funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.funcStep.RouteName)
		if e != nil {
			log.Print("print RunFuncStep error = ", e.Error())
		}
		output := FuncResult{funcJob, resp, nil, e}
		funcResults <- output
	}
	wg.Done()
}

func allocateFuncInner(req *http.Request, funcStep *FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, loopArray []interface{}, asyncMessage string, funcJobs chan FuncJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued allocateFuncInner: ", r)
		}
	}()
	loopCounter := 0
	for loopCounter < len(loopArray) {
		reqVars[funcStep.RouteName].LoopVars = loopArray[loopCounter]
		funcJob := FuncJob{loopCounter, req, funcStep, reqVars, resVars, asyncMessage}
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
	defer func() {
		if r := recover(); r != nil {
			log.Print("goroutine paniqued workerFuncInner: ", r)
			//output := FuncResult{nil, nil, nil, errors.New(r.(string))}
			//funcResults <- output
		}
	}()
	for funcJob := range funcJobs {
		resp, e := funcJob.funcStep.RunFuncStepInner(funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.funcStep.RouteName, funcJob.asyncMessage)
		if e != nil {
			log.Print("print RunFuncStepInner error = ", e.Error())
		}
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
