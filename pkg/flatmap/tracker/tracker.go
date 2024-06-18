package tracker

import (
	"log"
	"sync"

	"github.com/google/uuid"

	"github.com/numaproj/numaflow/pkg/isb"
)

type Tracker struct {
	requestMap sync.Map
	// TODO(stream): check perf between sync.map and mutex+map
	responseIdx sync.Map
	lock        sync.RWMutex
	m           map[string]isb.Offset
}

func NewTracker() *Tracker {
	return &Tracker{
		//requestMap:  sync.Map{},
		//responseIdx: sync.Map{},
		m: make(map[string]isb.Offset),
	}
}

func GetNewId() string {
	id, _ := uuid.NewUUID()
	return id.String()
}

//func (t *Tracker) AddRequest(msg *isb.ReadMessage) string {
//	// TODO(stream): we could use read offset as the ID now instead of UUID?
//	id := GetNewId()
//	//t.requestMap.Store(id, msg)
//	t.Set(id, msg)
//	return id
//}

func (t *Tracker) AddRequest(msg *isb.ReadMessage) {
	// TODO(stream): we could use read offset as the ID now instead of UUID?
	//id := GetNewId()
	id := msg.ReadOffset.String()
	//flatmapRequest := &types.RequestFlatmap{
	//	Request:    msg,
	//	Uid:        id,
	//	ReadOffset: msg.ReadOffset,
	//}
	//t.requestMap.Store(id, msg)
	t.Set(id, msg.ReadOffset)
}

//func (t *Tracker) GetRequest(id string) (*types.RequestFlatmap, bool) {
//	return t.Get(id)
//}

func (t *Tracker) NewResponse(id string) {
	t.responseIdx.Store(id, 1)
}

func (t *Tracker) IncrementRespIdx(id string) bool {
	idx, ok := t.responseIdx.Load(id)
	if !ok {
		return ok
	}
	newIdx := idx.(int) + 1
	t.responseIdx.Store(id, newIdx)
	return true
}

func (t *Tracker) GetIdx(id string) (int, bool) {
	idx, ok := t.responseIdx.Load(id)
	if !ok {
		return -1, ok
	}
	return idx.(int), ok
}

func (t *Tracker) RemoveRequest(id string) {
	//t.requestMap.Delete(id)
	//t.responseIdx.Delete(id)
	t.Delete(id)
}

func (t *Tracker) PrintAll() {
	t.lock.RLock()
	defer t.lock.RUnlock()
	for k, v := range t.m {
		log.Println("MYDEBUG: MAP VALS", k, " ", v)
	}
}

func (t *Tracker) Get(key string) (isb.Offset, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	item, ok := t.m[key]
	return item, ok
}

func (t *Tracker) Set(key string, val isb.Offset) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.m[key] = val
}

func (t *Tracker) Delete(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.m, key)
}

func (t *Tracker) GetItems() []isb.Offset {
	t.lock.Lock()
	defer t.lock.Unlock()
	items := make([]isb.Offset, 0, len(t.m))
	for _, val := range t.m {
		items = append(items, val)
	}
	return items
}

func (t *Tracker) Clear() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.m = make(map[string]isb.Offset)
}
