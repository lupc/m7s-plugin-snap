package snap

import (
	"bytes"
	"sync"
	"time"
)

type SubManager struct {
	subs map[string]*SnapSubscriber
}

func (s *SubManager) checkStop() {
	for {
		if s.subs != nil {
			for _, sub := range s.subs {
				if conf.Expire > 0 && time.Since(sub.lastRequestTime) > conf.Expire {
					sub.StopSub()
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (s *SubManager) GetOrCreate(streamPath string) *SnapSubscriber {

	if s.subs == nil {
		s.subs = make(map[string]*SnapSubscriber)
		go s.checkStop()
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
