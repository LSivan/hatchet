package workgroup

import (
	"fmt"
	xerror "github.com/LSivan/hatchet/x-error"
	xlog "github.com/LSivan/hatchet/x-log"
	"sync"
	"time"
)

type Handler func() error

type WorkGroup struct {
	buffer  []chan Handler
	isStart bool
	idxGen  func() int
}

func (wg *WorkGroup) SetIdxGen(gen func() int) {
	wg.idxGen = gen
}

func (wg *WorkGroup) Start() {
	wg.isStart = true
	wg.work()

}
func (wg *WorkGroup) Stop() {
	wg.isStart = false
}

func (wg *WorkGroup) Flush() {
	// 将所有的任务执行完毕
	wg.workWithWait()
}

func (wg *WorkGroup) AddWork(work Handler) error {
	if !wg.isStart {
		return fmt.Errorf("work group is stop so exec immediatly.:%v", work())
	}
	select {
	case wg.buffer[wg.idxGen()] <- work:
		return nil
		//default:
		//	return fmt.Errorf("work is full")
	}
}

// may be block the main routine
func (wg *WorkGroup) MustAddWork(work Handler) error {
	if !wg.isStart {
		return fmt.Errorf("work group is stop so exec immediatly.:%v", work())
	}
	wg.buffer[wg.idxGen()] <- work
	return nil
}

func (wg *WorkGroup) workWithWait() {
	var waitGroup sync.WaitGroup
	for i := 0; i < len(wg.buffer); i++ {
		waitGroup.Add(1)
		go func(j int) {
			defer waitGroup.Done()
			defer func() {
				if err := recover(); err != nil {
					xlog.Sugar.Named("WorkGroup work").Errorw("panic error", "err", fmt.Errorf("%v", err))
				}
			}()
			c := wg.buffer[j]
			for {
				select {
				case w := <-c:
					if w != nil {
						xerror.DoIfErrorNotNil(w(), func(err error) {
							xlog.Sugar.Named("WorkGroup work").Errorw("work return error", "err", err.Error())
						})
					}
				default:
					return
				}
			}
		}(i)
	}
	waitGroup.Wait()
}

func (wg *WorkGroup) work() {
	for i := 0; i < len(wg.buffer); i++ {
		go func(j int) {
			fmt.Println("begin consumer channel", j)
			defer func() {
				if err := recover(); err != nil {
					xlog.Sugar.Named("WorkGroup work").Errorw("occur error", "err", fmt.Errorf("%v", err))
				}
			}()
			c := wg.buffer[j]
			for {
				select {
				case w := <-c:
					if w != nil {
						xerror.DoIfErrorNotNil(w(), func(err error) {
							xlog.Sugar.Named("WorkGroup work").Errorw("work return error", "err", err.Error())
						})
					}
				}
			}
		}(i)
	}
}

func NewAsyncWorkGroup(workNum int, bufferSize int) *WorkGroup {
	wg := WorkGroup{
		buffer: make([]chan Handler, workNum),
		idxGen: func() int {
			return int(time.Now().Unix()) % workNum
		},
	}
	for i := 0; i < workNum; i++ {
		wg.buffer[i] = make(chan Handler, bufferSize)
	}
	return &wg
}
