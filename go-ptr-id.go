package djs

import (
	"sync"
)

type (
	fnGetPtrStore func(ctx uintptr)(*ptrStore)
	fnDelPtrStore func(ctx uintptr)
)

var (
	getPtrStore fnGetPtrStore
	delPtrStore fnDelPtrStore
)

func init() {
	getPtrStore, delPtrStore = InitPtrStore()
}

func InitPtrStore() (getPtrStore fnGetPtrStore, delPtrStore fnDelPtrStore) {
	lock := &sync.Mutex{}
	stores := make(map[uintptr]*ptrStore)

	getPtrStore = func(ctx uintptr)(*ptrStore) {
		lock.Lock()
		defer lock.Unlock()
		if store, ok := stores[ctx]; ok {
			return store
		}
		store := newPtrStore()
		stores[ctx] = store
		return store
	}

	delPtrStore = func(ctx uintptr) {
		lock.Lock()
		defer lock.Unlock()
		if store, ok := stores[ctx]; ok {
			store.clear()
		}
		delete(stores, ctx)
	}

	return
}

type (
	ref struct {
		ptr interface{}
		count int
	}
	ptrStore struct {
		lock *sync.Mutex
		index int
		id2ptr map[int]*ref
		ptr2id map[interface{}]int
	}
)

func newPtrStore() *ptrStore {
	return &ptrStore{
		lock: &sync.Mutex{},
		id2ptr: make(map[int]*ref),
		ptr2id: make(map[interface{}]int),
	}
}

func (s *ptrStore) register(i interface{}) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	if index, ok := s.ptr2id[i]; ok {
		ref, _ := s.id2ptr[index]
		ref.count += 1
		return index
	}

	s.index++
	s.id2ptr[s.index] = &ref{ptr:i, count:1}
	s.ptr2id[i] = s.index
	return s.index
}

func (s *ptrStore) lookup(i int) (ptr interface{}, ok bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if ref, ok1 := s.id2ptr[i]; ok1 {
		ptr, ok = ref.ptr, true
	}
	return
}

func (s *ptrStore) remove(i int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if ref, ok := s.id2ptr[i]; ok {
		ref.count -= 1
		if ref.count <= 0 {
			delete(s.id2ptr, i)
			delete(s.ptr2id, ref.ptr)
		}
	}
}

func (s *ptrStore) clear() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.id2ptr = nil
	s.ptr2id = nil
}
