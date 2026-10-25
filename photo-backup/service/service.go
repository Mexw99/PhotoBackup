package service

import (
	"context"
	"os"
	"sync"
	"time"

	"photo-backup/analysis"
	"photo-backup/repository"
)

// Service coordinates business operations without depending on the Wails runtime.
type Service struct {
	ctx            context.Context
	store          repository.Store
	removeSource   func(string) error
	analyzer       PhotoAnalyzer
	analysisCtx    context.Context
	analysisCancel context.CancelFunc
	analysisJobs   chan uint
	analysisWG     sync.WaitGroup
	analysisMu     sync.Mutex
	analysisQueued map[uint]struct{}
	analysisClosed bool
	notifyAnalysis func(AnalysisProgress)
}

func New(store repository.Store) *Service {
	return NewWithAnalyzer(store, analysis.NewVisionClientFromEnv())
}

func NewWithAnalyzer(store repository.Store, analyzer PhotoAnalyzer) *Service {
	analysisCtx, analysisCancel := context.WithCancel(context.Background())
	s := &Service{
		ctx:            context.Background(),
		store:          store,
		removeSource:   os.Remove,
		analyzer:       analyzer,
		analysisCtx:    analysisCtx,
		analysisCancel: analysisCancel,
		analysisJobs:   make(chan uint, 4096),
		analysisQueued: make(map[uint]struct{}),
	}
	if analyzer != nil {
		s.startAnalysisWorkers(3)
		go s.resumePendingAnalysis()
	}
	return s
}

// SetContext is called once during Wails startup before frontend requests begin.
func (s *Service) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *Service) Close() {
	if s == nil || s.analysisCancel == nil {
		return
	}
	s.analysisMu.Lock()
	if s.analysisClosed {
		s.analysisMu.Unlock()
		return
	}
	s.analysisClosed = true
	s.analysisMu.Unlock()
	s.analysisCancel()
	s.analysisWG.Wait()
}

func (s *Service) operation() (repository.Store, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	return s.store.WithContext(ctx), cancel
}
