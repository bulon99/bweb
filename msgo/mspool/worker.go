package mspool

import (
	msLog "github.com/bulon99/msgo/log"
	"time"
)

type Worker struct {
	pool     *Pool
	task     chan func() //任务队列
	lastTime time.Time   //执行任务的最后的时间
}

func (w *Worker) run() {
	w.pool.incRunning()
	go w.running()
}

func (w *Worker) running() {
	defer func() {
		//f报错时，worker不会调用PutWorker放回worker(丢弃)，从而不会运行p.cond.Signal，导致运行中的waitIdleWorker函数阻塞在p.cond.Wait
		//此时是可以通过重新创建一个worker避免阻塞，还要对waitIdleWorker函数进行优化
		w.pool.decRunning() //对在运行的worker数量-1，允许重新创建一个worker
		w.pool.workerCache.Put(w)
		w.pool.cond.Signal() //仍然通知
		if err := recover(); err != nil {
			if w.pool.PanicHandler != nil { //若添加了错误处理办法，则调用
				w.pool.PanicHandler()
			} else { //否则记录到日志
				msLog.Default().Error(err)
			}
		}
	}()
	for f := range w.task {
		if f == nil { //读到nil停止
			w.pool.workerCache.Put(w)
			return
		}
		f()
		w.pool.PutWorker(w) //放回空闲的worker
	}
}
