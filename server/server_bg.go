package server

import (
	"strings"
	"sync/atomic"
	"time"
)

func (s *Server) backgroundRoutines() {

	go s.fetchSearchConfig(s.state.Config.ScraperURL)

	// initial state
	s.state.Torrents = s.engine.GetTorrents()
	s.state.Stats.System.loadStats()
	//collecting sys stats
	go func() {
		for {
			select {
			case <-s.syncConnected:
				if atomic.CompareAndSwapInt32(&(s.syncSemphor), 0, 1) {
					go s.tickerRoutine()
				}
			case <-s.engine.TsChanged: // task added/deleted
				s.state.Torrents = s.engine.GetTorrents()
				s.state.Push()
			}
		}
	}()

	// rss updater
	go func() {
		// skip if not configured
		if !strings.HasPrefix(s.state.Config.RssURL, "http") {
			return
		}

		for range time.Tick(30 * time.Minute) {
			s.updateRSS()
		}
	}()

	go s.engine.RestoreCacheDir()
	if err := s.engine.StartTorrentWatcher(); err != nil {
		log.Println(err)
	}
}

// stateRoutines watches the tasks / sys states
func (s *Server) tickerRoutine() {
	tk := time.NewTicker(10 * time.Second)
	defer tk.Stop()

	log.Println("[tickerRoutine] sync connected, ticking for 10s")
	var noConnCount uint
	for range tk.C {

		if s.state.NumConnections() == 0 {
			noConnCount++
		}
		if noConnCount > 6 { // 1mins
			atomic.StoreInt32(&(s.syncSemphor), 0)
			log.Println("[tickerRoutine] sync exit for no web connections")
			return
		}

		s.state.Stats.System.loadStats()
		s.state.Stats.ConnStat = s.engine.ConnStat()
		s.state.Push()
	}
}
