package dating

import (
    "context"
    "log"
    "time"
)

type Scheduler struct {
    service Service
}

func NewScheduler(service Service) *Scheduler {
    return &Scheduler{service: service}
}

func (s *Scheduler) Start(ctx context.Context) {
    // Daily hotpicks generation at 9 AM
    go s.runDaily(ctx, 9, 0, s.service.GenerateDailyHotpicks)
    
    // Date reminders every hour
    go s.runHourly(ctx, s.service.SendDateReminders)
    
    // Cleanup expired hotpicks daily at 2 AM
    go s.runDaily(ctx, 2, 0, s.service.CleanupExpiredHotpicks)
}

func (s *Scheduler) runDaily(ctx context.Context, hour, minute int, task func(context.Context) error) {
    for {
        now := time.Now()
        next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
        if now.After(next) {
            next = next.Add(24 * time.Hour)
        }
        
        timer := time.NewTimer(next.Sub(now))
        
        select {
        case <-timer.C:
            if err := task(ctx); err != nil {
                log.Printf("Scheduled task failed: %v", err)
            }
        case <-ctx.Done():
            timer.Stop()
            return
        }
    }
}

func (s *Scheduler) runHourly(ctx context.Context, task func(context.Context) error) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := task(ctx); err != nil {
                log.Printf("Hourly task failed: %v", err)
            }
        case <-ctx.Done():
            return
        }
    }
}