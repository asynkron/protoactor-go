package actor

// A priority queue is a sort of meta-queue that uses a queue per priority level.
// The underlying queues can be anything that implements the queue interface.
//
// Messages that implement the PriorityMessage interface (i.e. have a GetPriority
// method) will be consumed in priority order first, queue order second. So if a
// higher priority message arrives, it will jump to the front of the queue from
// the consumer's perspective.
//
// There are 8 priority levels (0-7) because having too many levels impacts
// performance. And 8 priority levels ought to be enough for anybody. ;)
// This means your GetPriority method should return int8s between 0 and 7. If any
// return values are higher or lower, they will be reset to 7 or 0, respectively.
//
// The default priority level is 4 for messages that don't implement PriorityMessage.
// If you want your message processed sooner than un-prioritized messages, have its
// GetPriority method return a larger int8 value.
// Likewise, if you'd like to de-prioritize your message, have its GetPriority method
// return an int8 less than 4.

const (
	priorityLevels  = 8
	DefaultPriority = int8(priorityLevels / 2)
)

type PriorityMessage interface {
	GetPriority() int8
}

type priorityQueue struct {
	priorityQueues []queue
}

func NewPriorityQueue(queueProducer func() queue) *priorityQueue {
	q := &priorityQueue{
		priorityQueues: make([]queue, priorityLevels),
	}

	for p := 0; p < priorityLevels; p++ {
		q.priorityQueues[p] = queueProducer()
	}

	return q
}

func (q *priorityQueue) Push(item interface{}) {
	itemPriority := DefaultPriority

	if priorityItem, ok := item.(PriorityMessage); ok {
		itemPriority = priorityItem.GetPriority()
		if itemPriority < 0 {
			itemPriority = 0
		}
		if itemPriority > priorityLevels-1 {
			itemPriority = priorityLevels - 1
		}
	}

	q.priorityQueues[itemPriority].Push(item)
}

func (q *priorityQueue) Pop() interface{} {
	for p := priorityLevels - 1; p >= 0; p-- {
		if item := q.priorityQueues[p].Pop(); item != nil {
			return item
		}
	}
	return nil
}
