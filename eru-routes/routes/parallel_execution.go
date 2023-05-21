package routes

import (
	"context"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
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
	strCond       string
}

type FuncResult struct {
	job          FuncJob
	response     *http.Response
	responseVars *TemplateVars
	responseErr  error
}

func worker(ctx context.Context, route *Route, wg *sync.WaitGroup, jobs chan Job, results chan Result) {
	logs.WithContext(ctx).Debug("worker - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in worker: ", r))
			//output := Result{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//results <- output
			wg.Done()
		}
	}()
	for job := range jobs {
		//currentJob = job
		resp, r, e := route.RunRoute(ctx, job.request, job.url, job.vars, job.async, job.asyncMsg)
		output := Result{job, resp, r, e}
		results <- output
	}
	wg.Done()
}

func createWorkerPool(ctx context.Context, route *Route, noOfWorkers int, jobs chan Job, results chan Result) {
	logs.WithContext(ctx).Debug("createWorkerPool - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPool: ", r))
			return
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go worker(ctx, route, &wg, jobs, results)
	}
	wg.Wait()
	close(results)
}

func allocate(ctx context.Context, req *http.Request, u string, vars *TemplateVars, loopArray []interface{}, jobs chan Job, async bool, asyncMsg string) {
	logs.WithContext(ctx).Debug("allocate - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocate: ", r))
		}
	}()
	loopCounter := 0
	logs.WithContext(ctx).Info(fmt.Sprint("printing loopArray from allocate = ", loopArray))
	logs.WithContext(ctx).Info(fmt.Sprint(len(loopArray)))
	for loopCounter < len(loopArray) {
		laVars := *vars
		laVars.LoopVar = loopArray[loopCounter]
		job := Job{loopCounter, req, u, &laVars, async, asyncMsg}
		jobs <- job
		loopCounter++
	}
	close(jobs)
}

func allocateFunc(ctx context.Context, req *http.Request, funcSteps map[string]*FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, funcJobs chan FuncJob, mainRouteName string, funcThread int, loopThread int) {
	logs.WithContext(ctx).Debug("allocateFunc - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocateFunc: ", r))
		}
	}()
	loopCounter := 0
	for _, fs := range funcSteps {
		funcJob := FuncJob{loopCounter, req, fs, reqVars, resVars, "", mainRouteName, funcThread, loopThread, "true"}
		funcJobs <- funcJob
		loopCounter++
	}
	logs.WithContext(ctx).Info(fmt.Sprint("len(funcJobs) allocateFunc= ", len(funcJobs)))
	close(funcJobs)
}
func createWorkerPoolFunc(ctx context.Context, noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	logs.WithContext(ctx).Debug("createWorkerPoolFunc - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPoolFunc: ", r))
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go workerFunc(ctx, &wg, funcJobs, funcResults)
	}
	wg.Wait()
	close(funcResults)
}
func workerFunc(ctx context.Context, wg *sync.WaitGroup, funcJobs chan FuncJob, funcResults chan FuncResult) {
	logs.WithContext(ctx).Debug("workerFunc - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in workerFunc: ", r))
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
		resp, e := funcJob.funcStep.RunFuncStep(ctx, funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.mainRouteName, funcJob.funcThread, funcJob.loopThread)
		if e != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("print RunFuncStep error = ", e.Error()))
		}
		output := FuncResult{funcJob, resp, nil, e}
		funcResults <- output
	}
	wg.Done()
}

func allocateFuncInner(ctx context.Context, req *http.Request, funcStep *FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, loopArray []interface{}, asyncMessage string, funcJobs chan FuncJob, mainRouteName string, funcThread int, loopThread int, strCond string) {
	logs.WithContext(ctx).Debug("allocateFuncInner - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocateFuncInner: ", r))
		}
	}()
	loopCounter := 0
	logs.WithContext(ctx).Info(fmt.Sprint("len(loopArray) from allocateFuncInner = ", len(loopArray)))
	for loopCounter < len(loopArray) {
		reqVarsI, _ := cloneInterface(ctx, reqVars)
		reqVarsClone, _ := reqVarsI.(map[string]*TemplateVars)
		reqVarsClone[funcStep.GetRouteName()].LoopVar = loopArray[loopCounter]
		funcJob := FuncJob{loopCounter, req, funcStep, reqVarsClone, resVars, asyncMessage, mainRouteName, funcThread, loopThread, strCond}
		funcJobs <- funcJob
		loopCounter++
	}
	close(funcJobs)
}

func createWorkerPoolFuncInner(ctx context.Context, noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	logs.WithContext(ctx).Debug("createWorkerPoolFuncInner - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPoolFuncInner: ", r))
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go workerFuncInner(ctx, &wg, funcJobs, funcResults)
	}
	wg.Wait()
	close(funcResults)
}
func workerFuncInner(ctx context.Context, wg *sync.WaitGroup, funcJobs chan FuncJob, funcResults chan FuncResult) {
	logs.WithContext(ctx).Debug("workerFuncInner - Start")
	logs.WithContext(ctx).Info(fmt.Sprint("len(funcJobs) = ", len(funcJobs)))
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in workerFuncInner: ", r))
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
		resp, e := funcJob.funcStep.RunFuncStepInner(ctx, funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.mainRouteName, funcJob.asyncMessage, funcJob.funcThread, funcJob.loopThread, funcJob.strCond)
		if e != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("print RunFuncStepInner error = ", e.Error()))
		}
		output := FuncResult{funcJob, resp, nil, e}
		funcResults <- output
	}
	wg.Done()
}
