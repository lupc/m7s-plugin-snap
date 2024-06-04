package snap

import (
	"bytes"
	"sync"
	"time"
)

type SubManager struct {
	subs sync.Map
}

var subManager *SubManager

func GetSubManager() *SubManager {
	if subManager == nil {
		subManager = &SubManager{}
		subManager.goCheckStop()
	}
	return subManager
}

func (s *SubManager) goCheckStop() {

	go func() {
		defer func() {
			err := recover() //内置函数，可以捕捉到函数异常
			if err != nil {
				//这里是打印错误，还可以进行报警处理，例如微信，邮箱通知
				plugin.Logger.Sugar().Errorf("snap checkStop（协程）err：%v", err)
			}
		}()

		for {

			s.subs.Range(func(k any, v any) (isBreak bool) {
				sub, ok := v.(*SnapSubscriber)
				if ok && conf.Expire > 0 && time.Since(sub.lastRequestTime) > conf.Expire {
					sub.StopSub()
				}
				return true
			})

			time.Sleep(1 * time.Second)
		}
	}()

}

func (s *SubManager) GetOrCreate(streamPath string) *SnapSubscriber {

	// if s.subs == nil {
	// 	s.subs = new(sync.Map)
	// 	go s.checkStop()
	// }

	v, ok := s.subs.Load(streamPath)
	var sub *SnapSubscriber
	if !ok {
		sub = &SnapSubscriber{}
		sub.ID = streamPath
		sub.StreamPath = streamPath
		sub.lastPicBuffer = bytes.Buffer{}
		sub.SetIO(&sub.lastPicBuffer)
		sub.isSub = false
		sub.snapComplete = make(chan bool)
		sub.bufferLocker = &sync.RWMutex{}
		// s.subs[streamPath] = sub
		s.subs.Store(streamPath, sub)
	} else {
		sub2, ok2 := v.(*SnapSubscriber)
		if ok2 {
			sub = sub2
		}
	}
	return sub
}

func (s *SubManager) Remove(streamPath string) {
	// delete(s.subs, streamPath)
	s.subs.Delete(streamPath)
}
