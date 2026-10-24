package service

import (
	"context"
	"os"
	"sync"
	"time"

	"photo-backup/analysis"
	"photo-backup/repository"
)

type ImageAnalyzer interface {
	Analyze(string) (analysis.AIResult, error)
}

// Service coordinates business operations without depending on the Wails runtime.
type Service struct {
	ctx          context.Context
	store        repository.Store
	removeSource func(string) error
	analyzer     ImageAnalyzer
	queueMu      sync.Mutex
	queueCancel  context.CancelFunc
	queueWG      sync.WaitGroup
	aiProgress   func(AIQueueStatus)
}

func New(store repository.Store) *Service {
	return &Service{
		ctx:          context.Background(),
		store:        store,
		removeSource: os.Remove,
		analyzer:     analysis.NewAnalyzerFromEnv(),
	}
}

// SetImageAnalyzer is useful for tests and lets the application replace the AI provider.
func (s *Service) SetImageAnalyzer(analyzer ImageAnalyzer) {
	s.analyzer = analyzer
	s.startAIQueue()
}

// SetContext is called once during Wails startup before frontend requests begin.
func (s *Service) SetContext(ctx context.Context) {
	s.ctx = ctx
	s.startAIQueue()
}

// SetAIProgressCallback เชื่อมสถานะ Queue กับชั้น Wails โดยไม่ให้ service ผูกกับ Wails
func (s *Service) SetAIProgressCallback(callback func(AIQueueStatus)) { s.aiProgress = callback }

// Shutdown หยุด Worker ก่อนปิดฐานข้อมูล เพื่อไม่ให้มี transaction ค้างอยู่
func (s *Service) Shutdown() {
	s.queueMu.Lock()
	if s.queueCancel != nil {
		s.queueCancel()
		s.queueCancel = nil
	}
	s.queueMu.Unlock()
	s.queueWG.Wait()
	if closer, ok := s.analyzer.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func (s *Service) startAIQueue() {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if s.queueCancel != nil || s.analyzer == nil || s.ctx == nil {
		return
	}
	queueContext, cancel := context.WithCancel(s.ctx)
	s.queueCancel = cancel
	s.queueWG.Add(1)
	go func() {
		defer s.queueWG.Done()
		s.runAIQueue(queueContext)
	}()
}

func (s *Service) operation() (repository.Store, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	return s.store.WithContext(ctx), cancel
}
