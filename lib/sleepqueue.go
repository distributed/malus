package malus


import "time"


// interface sleepqueue?


type sleepRequest struct {
	howlong  uint64
	waketime int64
	retchan  chan bool
}

type ForwardSleepQueue struct {
	reqchan  chan *sleepRequest
	quitchan chan bool
	q        chan *sleepRequest
}


func (q *ForwardSleepQueue) Sleep(howlong uint64) {
	req := new(sleepRequest)
	req.howlong = howlong
	req.retchan = make(chan bool)

	q.reqchan <- req

	<-req.retchan
}


func (q *ForwardSleepQueue) server() {
	for {
		select {
		case req := <-q.reqchan:
			req.waketime = time.Nanoseconds() + int64(req.howlong)
			q.q <- req
		case <-q.quitchan:
			return
		}
	}
}

func (q *ForwardSleepQueue) sleeper() {
	for {
		select {
		case req := <-q.reqchan:
			sleeptime := req.waketime - time.Nanoseconds()
			for sleeptime > 0 {
				time.Sleep(sleeptime)
				sleeptime = req.waketime - time.Nanoseconds()
			}
			req.retchan <- true
		case <-q.quitchan:
			return
		}
	}
}

func (q *ForwardSleepQueue) Run() {
	go q.server()
	go q.sleeper()
}


func (q *ForwardSleepQueue) Stop() {
	q.quitchan <- true
	q.quitchan <- true
}


func NewSleepQueue() (q *ForwardSleepQueue) {
	q = new(ForwardSleepQueue)
	q.reqchan = make(chan *sleepRequest)
	q.quitchan = make(chan bool)
	q.q = make(chan *sleepRequest)

	go q.Run()

	return
}
