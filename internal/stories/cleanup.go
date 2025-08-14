package stories

import (
    "context"
    "log"
    "os"
    "time"
)

type CleanupService struct {
    service  Service
    interval time.Duration
}

func NewCleanupService(service Service) *CleanupService {
    // Get cleanup interval from env or default to 1 hour
    intervalStr := os.Getenv("STORY_CLEANUP_INTERVAL")
    interval := 1 * time.Hour
    
    if intervalStr != "" {
        if parsed, err := time.ParseDuration(intervalStr); err == nil {
            interval = parsed
        }
    }
    
    return &CleanupService{
        service:  service,
        interval: interval,
    }
}

// Start begins the cleanup job
func (c *CleanupService) Start(ctx context.Context) {
    log.Printf("Starting story cleanup service with interval: %v", c.interval)
    
    // Run initial cleanup
    c.runCleanup(ctx)
    
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            c.runCleanup(ctx)
        case <-ctx.Done():
            log.Println("Stopping story cleanup service")
            return
        }
    }
}

// runCleanup performs the actual cleanup
func (c *CleanupService) runCleanup(ctx context.Context) {
    log.Println("Running story cleanup...")
    
    startTime := time.Now()
    if err := c.service.CleanupExpiredStories(ctx); err != nil {
        log.Printf("Failed to cleanup expired stories: %v", err)
    } else {
        duration := time.Since(startTime)
        log.Printf("Story cleanup completed in %v", duration)
    }
}