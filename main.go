package snap

import (
	"bytes"
	_ "embed"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/config"
	"m7s.live/engine/v4/log"
	"m7s.live/engine/v4/util"
)

//go:embed default.yaml
var defaultYaml DefaultYaml

type SnapSubscriber struct {
	Subscriber
	StreamPath      string        //流路径
	snapComplete    chan bool     //抓拍完成信号
	lastSnapTime    time.Time     //最后抓拍时间
	lastRequestTime time.Time     //最后请求时间
	isSub           bool          //是否已订阅
	lastPicBuffer   bytes.Buffer  //最后一张图片数据
	bufferLocker    *sync.RWMutex //图片数据读写锁
}

func (s *SnapSubscriber) StopSub() {
	s.Stop(zap.String("reason", "snap"))
	s.snapComplete = nil
	subManager.Remove(s.StreamPath)
}

type SnapConfig struct {
	DefaultYaml
	config.Subscribe
	config.HTTP
	FFmpeg string        // ffmpeg的路径
	Path   string        //存储路径
	Filter string        //过滤器
	Expire time.Duration //抓拍订阅者过期时间，超过指定时间没有收到抓拍请求则停止订阅，0永不停止

}

func (snap *SnapConfig) OnEvent(event any) {

}

var conf = &SnapConfig{
	DefaultYaml: defaultYaml,
}

var plugin = InstallPlugin(conf)
var subManager = &SubManager{}

func (snap *SnapConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	streamPath := strings.TrimPrefix(r.URL.Path, "/")
	if r.URL.RawQuery != "" {
		streamPath += "?" + r.URL.RawQuery
	}
	var q = r.URL.Query()
	var expire, _ = strconv.Atoi(q.Get("expire")) //过期时长，请求时间减最后抓拍时间大于过期时长则等待下一个抓拍，毫秒
	w.Header().Set("Content-Type", "image/jpeg")
	sub := subManager.GetOrCreate(streamPath)
	var reqTime = time.Now()
	sub.lastRequestTime = reqTime
	// sub.ID = r.RemoteAddr
	// sub.SetParentCtx(r.Context())
	// sub.SetIO(w)

	if !sub.isSub {
		sub.isSub = true
		//子线程启动订阅
		go func() {
			if err := plugin.SubscribeBlock(streamPath, sub, SUBTYPE_RAW); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				sub.StopSub()
			}
		}()
	}

	var ok bool = true
	if sub.lastSnapTime.IsZero() {
		log.Debugf("%v首次抓拍http等待", sub.StreamPath)
		ok = <-sub.snapComplete //首次等待抓拍完成
		log.Debugf("%v首次抓拍http完成", sub.StreamPath)
	}

	if expire > 0 && reqTime.Sub(sub.lastSnapTime).Milliseconds() > int64(expire) {
		//等待下一个抓拍完成
		for {
			if sub.lastSnapTime.After(reqTime) {
				break
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	if ok {
		sub.bufferLocker.RLock()
		log.Debugf("最后抓拍图片：%v", sub.lastPicBuffer.Len())
		w.Write(sub.lastPicBuffer.Bytes()) //返回最后抓拍的图片
		sub.bufferLocker.RUnlock()
	} else {
		http.Error(w, "抓拍失败！", http.StatusBadRequest)
	}
}

func (s *SnapSubscriber) OnEvent(event any) {
	switch v := event.(type) {
	case VideoFrame:
		if v.IFrame {
			// s.Stop(zap.String("reason", "snap"))
			// var path = fmt.Sprintf("tmp/%v.jpg", time.Now().UnixMicro())
			var errOut util.Buffer
			firstFrame := v.GetAnnexB()
			// s.SetAnnexB(v)
			s.bufferLocker.Lock()
			s.lastPicBuffer.Reset()
			cmd := exec.Command(conf.FFmpeg, "-hide_banner", "-i", "pipe:0", "-vframes", "1", "-f", "mjpeg", "pipe:1")
			cmd.Stdin = &firstFrame
			cmd.Stderr = &errOut
			cmd.Stdout = &s.lastPicBuffer
			cmd.Run()

			if errOut.CanRead() {
				s.Debug(string(errOut))
			}
			if s.lastSnapTime.IsZero() {
				log.Debugf("%v首次抓拍ffmpeg完成", s.StreamPath)
				s.snapComplete <- true
			}
			s.lastSnapTime = time.Now()
			s.bufferLocker.Unlock()
		}
		if conf.Expire > 0 && time.Since(s.lastRequestTime) > conf.Expire {
			s.StopSub()
		}

	default:
		s.Subscriber.OnEvent(event)
	}
}
