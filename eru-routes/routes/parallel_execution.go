package routes

import (
	"log"
	"net/http"
	"sync"
)

type Job struct {
	id      int
	request *http.Request
	url     string
	vars    *TemplateVars
}
type Result struct {
	job          Job
	response     *http.Response
	responseVars *TemplateVars
	responseErr  error
}
type FuncJob struct {
	id       int
	request  *http.Request
	funcStep *FuncStep
	reqVars  map[string]*TemplateVars
	resVars  map[string]*TemplateVars
}
type FuncResult struct {
	job          FuncJob
	response     *http.Response
	responseVars *TemplateVars
	responseErr  error
}

func worker(route *Route, wg *sync.WaitGroup, jobs chan Job, results chan Result) {
	for job := range jobs {
		resp, r, e := route.RunRoute(job.request, job.url, job.vars)
		output := Result{job, resp, r, e}
		results <- output
	}
	wg.Done()
}

func createWorkerPool(route *Route, noOfWorkers int, jobs chan Job, results chan Result) {
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go worker(route, &wg, jobs, results)
	}
	wg.Wait()
	close(results)
}

func allocate(req *http.Request, u string, vars *TemplateVars, loopArray []interface{}, jobs chan Job) {
	loopCounter := 0
	for loopCounter < len(loopArray) {
		laVars := *vars
		laVars.LoopVars = loopArray[loopCounter]
		job := Job{loopCounter, req, u, &laVars}
		jobs <- job
		loopCounter++
	}
	log.Print("len(jobs) = ", len(jobs))
	close(jobs)
}

func allocateFunc(req *http.Request, funcSteps map[string]*FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, funcJobs chan FuncJob) {
	loopCounter := 0
	for _, fs := range funcSteps {
		funcJob := FuncJob{loopCounter, req, fs, reqVars, resVars}
		funcJobs <- funcJob
		loopCounter++
	}
	log.Print("len(funcJobs) = ", len(funcJobs))
	close(funcJobs)
}
func createWorkerPoolFunc(noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go workerFunc(&wg, funcJobs, funcResults)
	}
	wg.Wait()
	close(funcResults)
}
func workerFunc(wg *sync.WaitGroup, funcJobs chan FuncJob, funcResults chan FuncResult) {
	for funcJob := range funcJobs {
		resp, e := funcJob.funcStep.RunFuncStep(funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.funcStep.RouteName)
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
