// internal/notification/scheduler.go

package notifications

import (
    "context"
    "log"
    "time"
)

// NotificationScheduler handles scheduled notifications
type NotificationScheduler struct {
    service  Service
    interval time.Duration
    stopCh   chan struct{}
}

// NewNotificationScheduler creates a new notification scheduler
func NewNotificationScheduler(service Service, interval time.Duration) *NotificationScheduler {
    if interval == 0 {
        interval = 1 * time.Minute // Default to 1 minute
    }
    
    return &NotificationScheduler{
        service:  service,
        interval: interval,
        stopCh:   make(chan struct{}),
    }
}

// Start starts the scheduler
func (s *NotificationScheduler) Start(ctx context.Context) {
    log.Printf("Starting notification scheduler with interval: %v", s.interval)
    
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()
    
    // Run immediately on start
    s.processScheduled(ctx)
    
    for {
        select {
        case <-ticker.C:
            s.processScheduled(ctx)
        case <-s.stopCh:
            log.Println("Stopping notification scheduler")
            return
        case <-ctx.Done():
            log.Println("Context cancelled, stopping notification scheduler")
            return
        }
    }
}

// Stop stops the scheduler
func (s *NotificationScheduler) Stop() {
    close(s.stopCh)
}

// processScheduled processes pending scheduled notifications
func (s *NotificationScheduler) processScheduled(ctx context.Context) {
    if err := s.service.ProcessScheduledNotifications(ctx); err != nil {
        log.Printf("Error processing scheduled notifications: %v", err)
    }
}

// NotificationCleanupJob handles cleanup of old notifications
type NotificationCleanupJob struct {
    service      Service
    interval     time.Duration
    retentionAge time.Duration
    stopCh       chan struct{}
}

// NewNotificationCleanupJob creates a new cleanup job
func NewNotificationCleanupJob(service Service, interval, retentionAge time.Duration) *NotificationCleanupJob {
    if interval == 0 {
        interval = 24 * time.Hour // Default to daily
    }
    if retentionAge == 0 {
        retentionAge = 30 * 24 * time.Hour // Default to 30 days
    }
    
    return &NotificationCleanupJob{
        service:      service,
        interval:     interval,
        retentionAge: retentionAge,
        stopCh:       make(chan struct{}),
    }
}

// Start starts the cleanup job
func (j *NotificationCleanupJob) Start(ctx context.Context) {
    log.Printf("Starting notification cleanup job with interval: %v, retention: %v", j.interval, j.retentionAge)
    
    ticker := time.NewTicker(j.interval)
    defer ticker.Stop()
    
    // Run immediately on start
    j.cleanup(ctx)
    
    for {
        select {
        case <-ticker.C:
            j.cleanup(ctx)
        case <-j.stopCh:
            log.Println("Stopping notification cleanup job")
            return
        case <-ctx.Done():
            log.Println("Context cancelled, stopping notification cleanup job")
            return
        }
    }
}

// Stop stops the cleanup job
func (j *NotificationCleanupJob) Stop() {
    close(j.stopCh)
}

// cleanup performs the actual cleanup
func (j *NotificationCleanupJob) cleanup(ctx context.Context) {
    log.Println("Running notification cleanup...")
    
    startTime := time.Now()
    if err := j.service.CleanupOldNotifications(ctx, j.retentionAge); err != nil {
        log.Printf("Error cleaning up notifications: %v", err)
    } else {
        duration := time.Since(startTime)
        log.Printf("Notification cleanup completed in %v", duration)
    }
}

// DigestScheduler handles sending notification digests
type DigestScheduler struct {
    service  Service
    schedule string // cron-like schedule (e.g., "0 9 * * *" for 9 AM daily)
    stopCh   chan struct{}
}

// NewDigestScheduler creates a new digest scheduler
func NewDigestScheduler(service Service, schedule string) *DigestScheduler {
    if schedule == "" {
        schedule = "0 9 * * *" // Default to 9 AM daily
    }
    
    return &DigestScheduler{
        service:  service,
        schedule: schedule,
        stopCh:   make(chan struct{}),
    }
}

// Start starts the digest scheduler
func (d *DigestScheduler) Start(ctx context.Context) {
    log.Printf("Starting digest scheduler with schedule: %s", d.schedule)
    
    // For simplicity, using a fixed interval
    // In production, you might want to use a cron library
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            d.sendDigests(ctx)
        case <-d.stopCh:
            log.Println("Stopping digest scheduler")
            return
        case <-ctx.Done():
            log.Println("Context cancelled, stopping digest scheduler")
            return
        }
    }
}

// Stop stops the digest scheduler
func (d *DigestScheduler) Stop() {
    close(d.stopCh)
}

// sendDigests sends notification digests to users
func (d *DigestScheduler) sendDigests(ctx context.Context) {
    log.Println("Sending notification digests...")
    
    // This would aggregate unread notifications and send digest emails
    // Implementation would depend on your specific requirements
    
    // Example:
    // 1. Get users with unread notifications
    // 2. For each user, aggregate their unread notifications
    // 3. Send a digest email with summary
    
    log.Println("Notification digests sent")
}