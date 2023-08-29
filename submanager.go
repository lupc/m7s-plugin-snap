package snap

import (
	"bytes"
	"sync"
)

type SubManager struct {
	subs map[string]*SnapSubscriber
}

func (s *SubManager) GetOrCreate(streamPath string) *SnapSubscriber {

	if s.subs == nil {
		s.subs = make(map[string]*SnapSubscriber)
	}

	sub, ok := s.subs[streamPath]
	if !ok {
		sub = &SnapSubscriber{}
		sub.ID = streamPath
		sub.StreamPath = streamPath
		sub.lastPicBuffer = bytes.Buffer{}
		sub.SetIO(&sub.lastPicBuffer)
		sub.isSub = false
		sub.snapComplete = make(chan bool)
		sub.bufferLocker = &sync.RWMutex{}
		s.subs[streamPath] = sub
	}
	return sub
}

func (s *SubManager) Remove(streamPath string) {
	delete(s.subs, streamPath)
}
