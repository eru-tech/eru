package functions

import (
	"context"
	"fmt"
	"github.com/eru-tech/eru/eru-events/events"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/jmoiron/sqlx"
	"net/http"
	"runtime/debug"
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
	id              int
	request         *http.Request
	funcStep        *FuncStep
	reqVars         map[string]*TemplateVars
	resVars         map[string]*TemplateVars
	asyncMessage    string
	mainRouteName   string
	funcThread      int
	loopThread      int
	strCond         string
	funcStepName    string
	endFuncStepName string
	started         bool
	fromAsync       bool
}

type FuncResult struct {
	job             FuncJob
	response        *http.Response
	responseVars    FuncTemplateVars
	responseVarsMap map[string]FuncTemplateVars
	responseErr     error
}

type EventJob struct {
	id    int
	event events.EventI
}

type EventResult struct {
	Job       EventJob
	EventMsgs []events.EventMsg
}

func worker(ctx context.Context, route *Route, wg *sync.WaitGroup, jobs chan Job, results chan Result) {
	logs.WithContext(ctx).Debug("worker - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in worker: ", r, " : ", string(debug.Stack())))
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
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPool: ", r, " : ", string(debug.Stack())))
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
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocate: ", r, " : ", string(debug.Stack())))
		}
	}()
	loopCounter := 0
	for loopCounter < len(loopArray) {
		laVars := *vars
		laVars.LoopVar = loopArray[loopCounter]
		job := Job{loopCounter, req, u, &laVars, async, asyncMsg}
		jobs <- job
		loopCounter++
	}
	close(jobs)
}

func allocateFunc(ctx context.Context, req *http.Request, funcSteps map[string]*FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, funcJobs chan FuncJob, mainRouteName string, funcThread int, loopThread int, funcStepName string, endFuncStepName string, started bool, fromAsync bool) {
	logs.WithContext(ctx).Debug("allocateFunc - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocateFunc: ", r, " : ", string(debug.Stack())))
		}
	}()
	loopCounter := 0
	for fk, fs := range funcSteps {
		childStart := false
		fs.FuncKey = fk
		if started || fk == funcStepName || funcStepName == "" {
			childStart = true
		}
		r, rErr := CloneRequest(ctx, req)
		if rErr != nil {
			logs.WithContext(ctx).Error(rErr.Error())
		}
		resVarsI, _ := cloneInterface(ctx, resVars)
		resVarsClone, _ := resVarsI.(map[string]*TemplateVars)

		reqVarsI, _ := cloneInterface(ctx, reqVars)
		reqVarsClone, _ := reqVarsI.(map[string]*TemplateVars)

		logs.WithContext(ctx).Info(fmt.Sprint(fs.FuncKey))
		for k, _ := range reqVarsClone {
			logs.WithContext(ctx).Info(fmt.Sprint(k))
		}
		for k, _ := range resVarsClone {
			logs.WithContext(ctx).Info(fmt.Sprint(k))
		}

		funcJob := FuncJob{loopCounter, r, fs, reqVarsClone, resVarsClone, "", mainRouteName, funcThread, loopThread, "true", funcStepName, endFuncStepName, childStart, fromAsync}
		funcJobs <- funcJob
		loopCounter++
	}
	close(funcJobs)
}
func createWorkerPoolFunc(ctx context.Context, noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	logs.WithContext(ctx).Debug("createWorkerPoolFunc - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPoolFunc: ", r, " : ", string(debug.Stack())))
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
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in workerFunc: ", r, " : ", string(debug.Stack())))
			//output := FuncResult{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//funcResults <- output
			wg.Done()
		}
	}()
	for funcJob := range funcJobs {
		//currentJob = funcJob
		logs.WithContext(ctx).Info(fmt.Sprint("funcJob.mainRouteName from ", funcJob.funcStep.FuncKey, " is ", funcJob.mainRouteName))
		if funcJob.mainRouteName == "" {
			funcJob.mainRouteName = funcJob.funcStep.FuncKey
		}
		resp, funcVars, e := funcJob.funcStep.RunFuncStep(ctx, funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.mainRouteName, funcJob.funcThread, funcJob.loopThread, funcJob.funcStepName, funcJob.endFuncStepName, funcJob.started, funcJob.fromAsync)
		if e != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("print RunFuncStep error = ", e.Error()))
		}
		cloneFuncVarsI, _ := cloneInterface(ctx, funcVars)
		cloneFuncVarsMap, _ := cloneFuncVarsI.(map[string]FuncTemplateVars)
		output := FuncResult{funcJob, resp, FuncTemplateVars{}, cloneFuncVarsMap, e}
		funcResults <- output
	}
	wg.Done()
}

func allocateFuncInner(ctx context.Context, req *http.Request, fs *FuncStep, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, loopArray []interface{}, asyncMessage string, funcJobs chan FuncJob, mainRouteName string, funcThread int, loopThread int, strCond string, funcStepName string, endFuncStepName string, started bool, fromAsync bool) {
	logs.WithContext(ctx).Debug("allocateFuncInner - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocateFuncInner: ", r, " : ", string(debug.Stack())))
		}
	}()
	logs.WithContext(ctx).Info(fmt.Sprint(loopArray))
	logs.WithContext(ctx).Info(fmt.Sprint(fs.FuncKey))
	loopCounter := 0
	for loopCounter < len(loopArray) {
		funcStep := fs
		var funcStepErr error
		if len(loopArray) > 1 {
			funcStep, funcStepErr = fs.Clone(ctx)
			if funcStepErr != nil {
				logs.WithContext(ctx).Error(funcStepErr.Error())
				return
			}
		}
		reqVarsI, _ := cloneInterface(ctx, reqVars)
		reqVarsClone, _ := reqVarsI.(map[string]*TemplateVars)
		if reqVarsClone[funcStep.GetRouteName()] == nil {
			reqVarsClone[funcStep.GetRouteName()] = &TemplateVars{}
		}
		reqVarsClone[funcStep.GetRouteName()].LoopVar = loopArray[loopCounter]

		if reqVarsClone[funcStep.FuncKey] == nil {
			reqVarsClone[funcStep.FuncKey] = &TemplateVars{}
		}
		if !fromAsync || (fromAsync && funcStep.FuncKey != funcStepName) {
			reqVarsClone[funcStep.GetRouteName()].LoopVar = loopArray[loopCounter]
			reqVarsClone[funcStep.FuncKey].LoopVar = loopArray[loopCounter]
		} else {
			logs.WithContext(ctx).Info(fmt.Sprint(reqVarsClone[funcStep.FuncKey].LoopVar))
		}
		resVarsI, _ := cloneInterface(ctx, resVars)
		resVarsClone, _ := resVarsI.(map[string]*TemplateVars)
		funcJob := FuncJob{loopCounter, req, funcStep, reqVarsClone, resVarsClone, asyncMessage, mainRouteName, funcThread, loopThread, strCond, funcStepName, endFuncStepName, started, fromAsync}
		funcJobs <- funcJob
		loopCounter++
	}
	close(funcJobs)
}

func createWorkerPoolFuncInner(ctx context.Context, noOfWorkers int, funcJobs chan FuncJob, funcResults chan FuncResult) {
	logs.WithContext(ctx).Debug("createWorkerPoolFuncInner - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPoolFuncInner: ", r, " : ", string(debug.Stack())))
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
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in workerFuncInner: ", r, " : ", string(debug.Stack())))
			//output := FuncResult{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//funcResults <- output
			wg.Done()
		}
	}()
	for funcJob := range funcJobs {
		//currentJob = funcJob
		if funcJob.mainRouteName == "" {
			funcJob.mainRouteName = funcJob.funcStep.FuncKey
		}

		//logs.WithContext(ctx).Info(fmt.Sprint("funcJob.funcStep.Route.TargetHosts = ", funcJob.funcStep.Route.TargetHosts))
		resp, funcVars, e := funcJob.funcStep.RunFuncStepInner(ctx, funcJob.request, funcJob.reqVars, funcJob.resVars, funcJob.mainRouteName, funcJob.asyncMessage, funcJob.funcThread, funcJob.loopThread, funcJob.strCond, funcJob.funcStepName, funcJob.endFuncStepName, funcJob.started, funcJob.fromAsync)
		utils.PrintResponseBody(ctx, resp, "printing from workerFuncInner after RunFuncStepInner")
		if e != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("print RunFuncStepInner error = ", e.Error()))
		}
		cloneFuncVarsI, _ := cloneInterface(ctx, funcVars)
		cloneFuncVars, _ := cloneFuncVarsI.(FuncTemplateVars)
		output := FuncResult{funcJob, resp, cloneFuncVars, nil, e}
		funcResults <- output
	}
	wg.Done()
}

func AllocateEvent(ctx context.Context, event events.EventI, eventJobs chan EventJob, noOfWorkers int) {
	logs.WithContext(ctx).Debug("allocateEvent - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in allocateEvent: ", r, " : ", string(debug.Stack())))
		}
	}()
	logs.WithContext(ctx).Info(fmt.Sprint("length of eventJobs in AllocateEvent is ", len(eventJobs)))
	for i := 0; i < noOfWorkers; i++ {
		eventJob := EventJob{i + 1, event}
		eventJobs <- eventJob
	}

	close(eventJobs)
}

func CreateWorkerPoolEvent(ctx context.Context, noOfWorkers int, eventJobs chan EventJob, eventResults chan EventResult, dbCon *sqlx.DB) {
	logs.WithContext(ctx).Debug("createWorkerPoolEvent - Start")
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in createWorkerPoolEvent: ", r, " : ", string(debug.Stack())))
		}
	}()
	var wg sync.WaitGroup

	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go WorkerEvent(ctx, &wg, eventJobs, eventResults, dbCon, i)
	}
	wg.Wait()
	close(eventResults)
}
func WorkerEvent(ctx context.Context, wg *sync.WaitGroup, eventJobs chan EventJob, eventResults chan EventResult, dbCon *sqlx.DB, wcnt int) {
	logs.WithContext(ctx).Info(fmt.Sprint("workerEvent - Start : ", wcnt))
	defer func() {
		if r := recover(); r != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in workerEvent: ", r, " : ", string(debug.Stack())))
			//output := FuncResult{currentJob, nil, nil, errors.New(fmt.Sprint(r))}
			//funcResults <- output
			wg.Done()
		}
	}()
	for eventJob := range eventJobs {
		logs.WithContext(ctx).Info(fmt.Sprint("polling starting for job worker ", wcnt))
		eventMsgs, e := eventJob.event.Poll(ctx)
		if e != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("print event.Poll error = ", e.Error()))
		}
		output := EventResult{eventJob, eventMsgs}
		logs.WithContext(ctx).Info(fmt.Sprint("count of messages recevied for job worker no : ", wcnt, " is ", len(eventMsgs)))
		eventResults <- output
	}
	wg.Done()
}
