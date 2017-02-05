package gocbcore

import (
	"strings"
	"sync"
	"time"
)

type memdInitFunc func(*memdPipeline, time.Time) error

type CloseHandler func(*memdPipeline)
type BadRouteHandler func(*memdPipeline, *memdQRequest, *memdResponse)

type Callback func(*memdResponse, *memdRequest, error)

type memdPipeline struct {
	lock sync.RWMutex

	queue *memdQueue

	address  string
	conn     memdReadWriteCloser
	isClosed bool
	ioDoneCh chan bool

	opList memdOpMap

	handleBadRoute BadRouteHandler
	handleDeath    CloseHandler
}

func CreateMemdPipeline(address string) *memdPipeline {
	return &memdPipeline{
		address:  address,
		queue:    createMemdQueue(),
		ioDoneCh: make(chan bool, 1),
	}
}

func (s *memdPipeline) Address() string {
	return s.address
}

func (s *memdPipeline) Hostname() string {
	return strings.Split(s.address, ":")[0]
}

func (s *memdPipeline) IsClosed() bool {
	s.lock.Lock()
	rv := s.isClosed
	s.lock.Unlock()
	return rv
}

func (s *memdPipeline) SetHandlers(badRouteFn BadRouteHandler, deathFn CloseHandler) {
	s.lock.Lock()

	if s.isClosed {
		// We died between authentication and here, immediately notify the deathFn
		s.lock.Unlock()
		deathFn(s)
		return
	}

	s.handleBadRoute = badRouteFn
	s.handleDeath = deathFn
	s.lock.Unlock()
}

func (pipeline *memdPipeline) ExecuteRequest(req *memdQRequest, deadline time.Time) (respOut *memdResponse, errOut error) {
	if req.Callback != nil {
		panic("Tried to synchronously dispatch an operation with an async handler.")
	}

	signal := make(chan bool)

	req.Callback = func(resp *memdResponse, _ *memdRequest, err error) {
		respOut = resp
		errOut = err
		signal <- true
	}

	if !pipeline.queue.QueueRequest(req) {
		return nil, ErrDispatchFail
	}

	timeoutTmr := AcquireTimer(deadline.Sub(time.Now()))
	select {
	case <-signal:
		ReleaseTimer(timeoutTmr, false)
		return
	case <-timeoutTmr.C:
		ReleaseTimer(timeoutTmr, true)
		if !req.Cancel() {
			<-signal
			return
		}
		return nil, ErrTimeout
	}
}

func (pipeline *memdPipeline) dispatchRequest(req *memdQRequest) error {
	// We do a cursory check of the server to avoid dispatching operations on the network
	//   that have already knowingly been cancelled.  This doesn't guarentee a cancelled
	//   operation from being sent, but it does reduce network IO when possible.
	if req.QueueOwner() != pipeline.queue {
		// Even though we failed to dispatch, this is not actually an error,
		//   we just consume the operation since its already been handled elsewhere
		return nil
	}

	pipeline.opList.Add(req)

	err := pipeline.conn.WritePacket(&req.memdRequest)
	if err != nil {
		logDebugf("Got write error")
		pipeline.opList.Remove(req)
		return err
	}

	return nil
}

func (s *memdPipeline) resolveRequest(resp *memdResponse) {
	opIndex := resp.Opaque
	isFailResp := resp.Magic == ResMagic && resp.Status != StatusSuccess

	// Find the request that goes with this response
	alwaysRemove := isFailResp
	req := s.opList.FindAndMaybeRemove(opIndex, alwaysRemove)

	if req == nil {
		// There is no known request that goes with this response.  Ignore it.
		logDebugf("Received response with no corresponding request.")
		return
	}

	if isFailResp || !req.Persistent {
		if !s.queue.UnqueueRequest(req) {
			// While we found a valid request, the request does not appear to be queued
			//   with this server anymore, this probably means that it has been cancelled.
			logDebugf("Received response for cancelled request.")
			return
		}
	}

	if isFailResp && resp.Status == StatusNotMyVBucket {
		// If possible, lets backchannel our NMV back to the Agent of this memdQueueConn
		//   instance.  This is primarily meant to enhance performance, and allow the
		//   agent to be instantly notified upon a new configuration arriving.  If the
		//   backchannel isn't available, we just Callback with the NMV error.
		logDebugf("Received NMV response.")
		s.lock.RLock()
		badRouteFn := s.handleBadRoute
		s.lock.RUnlock()
		if badRouteFn != nil {
			badRouteFn(s, req, resp)
			return
		}
	}

	// Call the requests callback handler...  Ignore Status field for incoming requests.
	logSchedf("Dispatching response callback. OP=0x%x. Opaque=%d", resp.Opcode, resp.Opaque)
	if resp.Magic == ReqMagic {
		req.Callback(resp, &req.memdRequest, nil)
	} else {
		req.Callback(resp, &req.memdRequest, getMemdError(resp.Status))
	}
}

func (pipeline *memdPipeline) ioLoop() {
	killSig := make(chan bool)

	// Reading
	go func() {
		logDebugf("Reader loop starting...")
		for {
			resp := &memdResponse{}
			err := pipeline.conn.ReadPacket(resp)
			if err != nil {
				logDebugf("Server read error: %v", err)
				killSig <- true
				break
			}
			logSchedf("Resolving response OP=0x%x. Opaque=%d", resp.Opcode, resp.Opaque)
			pipeline.resolveRequest(resp)
		}
	}()

	// Writing
	logDebugf("Writer loop starting...")
	for {
		select {
		case req := <-pipeline.queue.reqsCh:
			logSchedf("Dispatching request OP=0x%x. Opaque=%d.", req.Opcode, req.Opaque)
			err := pipeline.dispatchRequest(req)
			if err != nil {
				// Ensure that the connection gets fully closed
				pipeline.conn.Close()

				// We must wait for the receive goroutine to die as well before we can continue.
				<-killSig

				// We have to run this code in a goroutine as the requests channel
				//   may be full so we need to notify people to drain this server
				//   before it may complete, however at the same time, we have to
				//   wait to trip ioDoneCh until after that last request is returned
				//   to the queue or our drain might miss it.
				go func() {
					// Return the active request back to the queue and mark the io as being completed.
					pipeline.queue.reqsCh <- req

					// Now we must signal drainers that we are done!
					pipeline.ioDoneCh <- true
				}()

				return
			}
		case <-killSig:
			// Now we must signal drainers that we are done!
			pipeline.ioDoneCh <- true

			return
		}
	}
}

func (pipeline *memdPipeline) Run() {
	logDebugf("Beginning pipeline runner")

	// Run the IO loop.  This will block until the connection has been closed.
	pipeline.ioLoop()

	// Signal the creator that we died :(
	pipeline.lock.Lock()
	pipeline.isClosed = true
	deathFn := pipeline.handleDeath
	pipeline.lock.Unlock()
	if deathFn != nil {
		deathFn(pipeline)
	} else {
		pipeline.Drain(nil)
	}
}

func (pipeline *memdPipeline) Close() {
	pipeline.conn.Close()
}

func (pipeline *memdPipeline) Drain(reqCb drainedReqCallback) {
	// If the user does no pass a drain callback, we handle the requests
	//   by immediately failing them with a network error.
	if reqCb == nil {
		reqCb = func(req *memdQRequest) {
			req.Callback(nil, nil, ErrNetwork)
		}
	}

	// Drain the request queue, this will block until the io thread signals
	//   on ioDoneCh, and the queues have been completely emptied
	pipeline.queue.Drain(reqCb, pipeline.ioDoneCh)

	// As a last step, immediately notify all the requests that were
	//   on-the-wire that a network error has occurred.
	pipeline.opList.Drain(func(r *memdQRequest) {
		if pipeline.queue.UnqueueRequest(r) {
			r.Callback(nil, nil, ErrNetwork)
		}
	})
}
