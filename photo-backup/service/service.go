package service

import (
	"context"
	"os"
	"strconv"
	"strings"
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
		s.startAnalysisWorkers(analysisWorkerCount(analyzer))
		s.analysisWG.Add(1)
		go func() {
			defer s.analysisWG.Done()
			s.resumePendingAnalysis()
		}()
	}
	return s
}

func analysisWorkerCount(analyzer PhotoAnalyzer) int {
	count := analysisEnvInt("AI_ANALYSIS_WORKERS", 4)
	if count < 1 {
		return 1
	}
	if count > 8 {
		count = 8
	}
	if keyCounter, ok := analyzer.(interface{ APIKeyCount() int }); ok {
		keyCount := keyCounter.APIKeyCount()
		if keyCount > 0 && count > keyCount {
			count = keyCount
		}
	}
	return count
}

func analysisEnvInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}

// SetContext is called once during Wails startup before frontend requests begin.
func (s *Service) SetContext(ctx context.Context) {
	s.analysisMu.Lock()
	defer s.analysisMu.Unlock()
	s.ctx = ctx
}

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
	s.analysisMu.Lock()
	parent := s.ctx
	s.analysisMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	return s.store.WithContext(ctx), cancel
}
