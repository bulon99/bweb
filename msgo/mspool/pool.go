package mspool

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type sig struct {
}

const DefaultExpire = 3

var (
	ErrorInValidCap    = errors.New("pool cap can not <= 0")
	ErrorInValidExpire = errors.New("pool expire can not <= 0")
	ErrorHasClosed     = errors.New("pool has been released!!")
)

type Pool struct {
	cap          int32         //容量 pool max
	running      int32         //正在运行的worker的数量
	workers      []*Worker     //空闲worker
	expire       time.Duration //过期时间 空闲的worker超过这个时间 回收掉
	release      chan sig      //释放资源  pool就不能使用了
	lock         sync.Mutex    //去保护pool里面的相关资源的安全
	once         sync.Once
	workerCache  sync.Pool //worker缓存
	cond         *sync.Cond
	PanicHandler func()
}

func NewPool(cap int) (*Pool, error) {
	return NewTimePool(cap, DefaultExpire)
}

func NewTimePool(cap int, expire int) (*Pool, error) {
	if cap <= 0 {
		return nil, ErrorInValidCap
	}
	if expire <= 0 {
		return nil, ErrorInValidExpire
	}
	p := &Pool{
		cap:     int32(cap),
		expire:  time.Duration(expire) * time.Second,
		release: make(chan sig, 1),
	}
	p.workerCache.New = func() any {
		return &Worker{
			pool: p,
			task: make(chan func(), 1),
		}
	}
	p.cond = sync.NewCond(&p.lock) //参数是互斥锁即可
	go p.expireWorker()
	return p, nil
}

//提交任务
func (p *Pool) Submit(task func()) error {
	if len(p.release) > 0 {
		return ErrorHasClosed
	}
	//获取池里面的一个worker，然后执行任务就可以了
	w := p.GetWorker()
	w.task <- task
	return nil
}

func (p *Pool) GetWorker() *Worker {
	// 1. 目的获取pool里面的worker
	// 2. 如果 有空闲的worker 直接获取
	p.lock.Lock()
	idleWorkers := p.workers
	n := len(idleWorkers) - 1
	if n >= 0 { //有worker
		w := idleWorkers[n] //从尾部取worker
		idleWorkers[n] = nil
		p.workers = idleWorkers[:n]
		p.lock.Unlock()
		return w
	}
	// 3.如过没有空闲的worker，新建一个worker，前提是容量未满
	if p.running < p.cap {
		p.lock.Unlock()
		c := p.workerCache.Get()
		var w *Worker
		if c == nil {
			w = &Worker{
				pool: p,
				task: make(chan func(), 1),
			}
		} else {
			w = c.(*Worker)
		}
		w.run()
		return w
	}
	p.lock.Unlock()
	// 4. 如果正在运行的workers达到pool容量（没有空闲worker），阻塞等待，worker释放
	//for {
	//	p.lock.Lock()
	//	idleWorkers = p.workers
	//	n = len(idleWorkers) - 1
	//	if n < 0 {
	//		p.lock.Unlock()
	//		continue
	//	}
	//	w := idleWorkers[n]
	//	idleWorkers[n] = nil
	//	p.workers = idleWorkers[:n]
	//	p.lock.Unlock()
	//	return w
	//}

	//上面使用for循环等待空闲worker不优雅，使用sync.Cond优化
	return p.waitIdleWorker()
}

//func (p *Pool) waitIdleWorker() *Worker {
//	p.lock.Lock()
//	p.cond.Wait()
//	idleWorkers := p.workers
//	n := len(idleWorkers) - 1
//	if n < 0 {
//		p.lock.Unlock()
//		return p.waitIdleWorker()
//	}
//	w := idleWorkers[n]
//	idleWorkers[n] = nil
//	p.workers = idleWorkers[:n]
//	p.lock.Unlock()
//	return w
//}

func (p *Pool) waitIdleWorker() *Worker {
	p.lock.Lock()
	p.cond.Wait()
	idleWorkers := p.workers
	n := len(idleWorkers) - 1
	if n < 0 {
		p.lock.Unlock()
		if p.running < p.cap { //由于有的worker中任务可能出现异常，从而释放容量，检查是否可新建worker
			//还不够pool的容量，直接新建一个
			c := p.workerCache.Get()
			var w *Worker
			if c == nil {
				w = &Worker{
					pool: p,
					task: make(chan func(), 1),
				}
			} else {
				w = c.(*Worker)
			}
			w.run()
			return w
		}
		return p.waitIdleWorker()
	}
	w := idleWorkers[n]
	idleWorkers[n] = nil
	p.workers = idleWorkers[:n]
	p.lock.Unlock()
	return w
}

func (p *Pool) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

func (p *Pool) decRunning() {
	atomic.AddInt32(&p.running, -1)
}

func (p *Pool) PutWorker(w *Worker) { //放回空闲的worker
	w.lastTime = time.Now() //标记最后执行时间，用于检查，回收
	p.lock.Lock()
	p.workers = append(p.workers, w) //添加到尾部
	p.cond.Signal()                  //通知唤醒p.cond.Wait
	p.lock.Unlock()
}

func (p *Pool) Release() {
	p.once.Do(func() {
		//只执行一次
		p.lock.Lock()
		workers := p.workers
		for i, w := range workers {
			w.task = nil
			w.pool = nil
			workers[i] = nil
		}
		p.workers = nil
		p.lock.Unlock()
		p.release <- sig{}
	})
}

func (p *Pool) IsRelease() bool {
	return len(p.release) > 0
}

func (p *Pool) Restart() bool {
	if len(p.release) <= 0 {
		return true
	}
	_ = <-p.release
	return true
}

func (p *Pool) expireWorker() { //若空闲worker多了，进行清除
	//定时清理过期的空闲worker
	ticker := time.NewTicker(p.expire)
	for range ticker.C {
		if p.IsRelease() {
			break
		}
		//循环空闲的workers 如果当前时间和worker的最后运行任务的时间 差值大于expire 进行清理
		p.lock.Lock()
		idleWorkers := p.workers
		n := len(idleWorkers) - 1
		if n >= 0 {
			var clearN = -1
			for i, w := range idleWorkers {
				if time.Now().Sub(w.lastTime) <= p.expire {
					break
				}
				clearN = i
				w.task <- nil
				idleWorkers[i] = nil
			}
			if clearN != -1 {
				if clearN >= len(idleWorkers)-1 {
					p.workers = idleWorkers[:0]
				} else {
					p.workers = idleWorkers[clearN+1:]
				}
				fmt.Printf("清除完成,running:%d, workers:%v \n", p.running, p.workers)
			}
		}
		p.lock.Unlock()
	}
}

func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) Free() int {
	return int(p.cap - p.running)
}
