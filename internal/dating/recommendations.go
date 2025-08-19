// internal/dating/recommendations.go

package dating

import (
    "context"
    "encoding/json"
    "sort"
    "time"
)

type RecommendationEngine struct {
    service        Service
    matchingEngine MatchingEngine
    repo          Repository
}

func NewRecommendationEngine(service Service, engine MatchingEngine, repo Repository) *RecommendationEngine {
    return &RecommendationEngine{
        service:        service,
        matchingEngine: engine,
        repo:          repo,
    }
}

func (r *RecommendationEngine) GenerateDailyHotpicks(ctx context.Context) error {
    // Get all active users
    activeUsers, err := r.repo.GetActiveUsers(ctx, 30)
    if err != nil {
        return err
    }
    
    for _, user := range activeUsers {
        // Skip if already generated today
        hasToday, err := r.repo.HasTodayHotpicks(ctx, user.ID)
        if err != nil || hasToday {
            continue
        }
        
        // Get user preferences and profile
        userProfile, err := r.repo.GetUserProfile(ctx, user.ID)
        if err != nil {
            continue
        }
        
        // Find candidates
        candidates, err := r.findCandidates(ctx, user.ID, userProfile)
        if err != nil {
            continue
        }
        
        // Score and rank candidates
        scoredCandidates := r.scoreAndRank(ctx, userProfile, candidates)
        
        // Create hotpicks (top 10)
        for i, candidate := range scoredCandidates {
            if i >= 10 {
                break
            }
            
            factorsJSON, _ := json.Marshal(candidate.Factors)
            
            hotpick := &Hotpick{
                UserID:            user.ID,
                RecommendedUserID: candidate.UserID,
                Score:            candidate.Score,
                Reason:           &candidate.Reason,
                Factors:          factorsJSON,
                ExpiresAt:        ptr(time.Now().Add(24 * time.Hour)),
            }
            
            r.repo.CreateHotpick(ctx, hotpick)
        }
    }
    
    return nil
}

func (r *RecommendationEngine) findCandidates(ctx context.Context, userID int64, profile *UserProfile) ([]*UserProfile, error) {
    filters := &CandidateFilters{
        ExcludeMatched:    true,
        ExcludeBlocked:    true,
        ExcludeDeclined:   true,
        Gender:            derefString(profile.PreferredGender, ""),
        MinAge:            derefInt(profile.PreferredMinAge, 18),
        MaxAge:            derefInt(profile.PreferredMaxAge, 100),
        MaxDistance:       derefFloat64(profile.PreferredDistance, 100.0),
        Limit:             100,
    }
    
    return r.repo.FindCandidates(ctx, userID, filters)
}

func (r *RecommendationEngine) scoreAndRank(ctx context.Context, userProfile *UserProfile, candidates []*UserProfile) []*ScoredCandidate {
    scored := make([]*ScoredCandidate, 0, len(candidates))
    
    for _, candidate := range candidates {
        score, factors, _ := r.matchingEngine.CalculateCompatibility(ctx, userProfile, candidate)
        
        // Apply boosters
        score = r.applyBoosters(ctx, userProfile, candidate, score)
        
        // Generate reason
        reason := r.generateReason(factors, candidate)
        
        scored = append(scored, &ScoredCandidate{
            UserID:  candidate.ID,
            Profile: candidate,
            Score:   score,
            Factors: factors,
            Reason:  reason,
        })
    }
    
    // Sort by score
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })
    
    return scored
}

func (r *RecommendationEngine) applyBoosters(ctx context.Context, user, candidate *UserProfile, baseScore float64) float64 {
    score := baseScore
    
    // New user boost
    if time.Since(candidate.CreatedAt) < 7*24*time.Hour {
        score *= 1.2
    }
    
    // Active user boost
    if time.Since(candidate.LastActive) < 24*time.Hour {
        score *= 1.1
    }
    
    // Verified profile boost
    if candidate.IsVerified {
        score *= 1.15
    }
    
    // Complete profile boost
    if candidate.CompletionScore > 0.8 {
        score *= 1.1
    }
    
    // Mutual interests super boost
    mutualInterests := r.countMutualInterests(user.Interests, candidate.Interests)
    if mutualInterests >= 3 {
        score *= 1.3
    }
    
    return min(1.0, score)
}

func (r *RecommendationEngine) generateReason(factors *CompatibilityFactors, candidate *UserProfile) string {
    reasons := []string{}
    
    if factors.InterestsMatch > 0.7 {
        reasons = append(reasons, "shares your interests")
    }
    if factors.LocationProximity > 0.8 {
        reasons = append(reasons, "lives nearby")
    }
    if factors.PreferencesMatch > 0.8 {
        reasons = append(reasons, "looking for the same thing")
    }
    
    if len(reasons) == 0 {
        return "Recommended for you"
    }
    
    return candidate.DisplayName + " " + reasons[0]
}

func (r *RecommendationEngine) countMutualInterests(interests1, interests2 []string) int {
    interestMap := make(map[string]bool)
    for _, interest := range interests1 {
        interestMap[interest] = true
    }
    
    count := 0
    for _, interest := range interests2 {
        if interestMap[interest] {
            count++
        }
    }
    
    return count
}

// Helper functions
func ptr[T any](v T) *T {
    return &v
}

func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

// Helper functions for dereferencing pointers with defaults
func derefString(s *string, defaultValue string) string {
    if s != nil {
        return *s
    }
    return defaultValue
}

func derefInt(i *int, defaultValue int) int {
    if i != nil {
        return *i
    }
    return defaultValue
}

func derefFloat64(f *float64, defaultValue float64) float64 {
    if f != nil {
        return *f
    }
    return defaultValue
}