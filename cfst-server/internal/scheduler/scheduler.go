package scheduler

import (
	"log"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron *cron.Cron
}

type TaskFunc func()

func New() *Scheduler {
	return &Scheduler{
		cron: cron.New(),
	}
}

func (s *Scheduler) AddTask(schedule string, task TaskFunc) error {
	_, err := s.cron.AddFunc(schedule, func() {
		log.Println("[Scheduler] Running scheduled speed test...")
		task()
	})
	return err
}

func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("[Scheduler] Started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("[Scheduler] Stopped")
}
