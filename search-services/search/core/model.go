package core

import (
	"slices"
	"sync"
)

type Comics struct {
	ID    int
	URL   string
	Words []string
}

type Index struct {
	index map[string][]int
	lock  sync.RWMutex
}

func NewIndex() *Index {
	return &Index{
		index: make(map[string][]int),
	}
}

func (i *Index) GetIDs(word string) []int {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return slices.Clone(i.index[word])
}

func (i *Index) Add(id int, words []string) {
	i.lock.Lock()
	defer i.lock.Unlock()

	for _, word := range words {
		i.index[word] = append(i.index[word], id)
	}
}
