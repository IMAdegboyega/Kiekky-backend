package dating

import (
    "context"
    "math"
)

type MatchingEngine interface {
    CalculateCompatibility(ctx context.Context, user1Profile, user2Profile *UserProfile) (float64, *CompatibilityFactors, error)
    GenerateRecommendations(ctx context.Context, userProfile *UserProfile, candidates []*UserProfile) ([]*ScoredCandidate, error)
    UpdateUserFactors(ctx context.Context, userID int64, interactions []*UserInteraction) error
}

type matchingEngine struct {
    repo Repository
}

func NewMatchingEngine(repo Repository) MatchingEngine {
    return &matchingEngine{repo: repo}
}

func (m *matchingEngine) CalculateCompatibility(ctx context.Context, user1, user2 *UserProfile) (float64, *CompatibilityFactors, error) {
    factors := &CompatibilityFactors{}
    
    // 1. Calculate interests match (30% weight)
    factors.InterestsMatch = m.calculateInterestsScore(user1.Interests, user2.Interests)
    
    // 2. Calculate location proximity (20% weight)
    factors.LocationProximity = m.calculateLocationScore(
        user1.Latitude, user1.Longitude,
        user2.Latitude, user2.Longitude,
    )
    
    // 3. Calculate age compatibility (15% weight)
    factors.AgeCompatibility = m.calculateAgeScore(user1.Age, user2.Age)
    
    // 4. Calculate activity alignment (15% weight)
    factors.ActivityAlignment = m.calculateActivityScore(
        user1.LastActive, user2.LastActive,
        user1.ActivityPattern, user2.ActivityPattern,
    )
    
    // 5. Calculate preferences match (10% weight)
    factors.PreferencesMatch = m.calculatePreferencesScore(
        user1.LookingFor, user2.LookingFor,
        user1.Gender, user2.Gender,
    )
    
    // 6. Profile completeness (5% weight)
    factors.ProfileCompleteness = (user1.CompletionScore + user2.CompletionScore) / 2
    
    // 7. Engagement level (5% weight)
    factors.EngagementLevel = m.calculateEngagementScore(
        user1.ResponseRate, user2.ResponseRate,
        user1.ActiveDays, user2.ActiveDays,
    )
    
    // Calculate weighted total
    totalScore := factors.InterestsMatch * 0.30 +
                  factors.LocationProximity * 0.20 +
                  factors.AgeCompatibility * 0.15 +
                  factors.ActivityAlignment * 0.15 +
                  factors.PreferencesMatch * 0.10 +
                  factors.ProfileCompleteness * 0.05 +
                  factors.EngagementLevel * 0.05
    
    return totalScore, factors, nil
}

func (m *matchingEngine) calculateInterestsScore(interests1, interests2 []string) float64 {
    if len(interests1) == 0 || len(interests2) == 0 {
        return 0.5
    }
    
    interestMap := make(map[string]bool)
    for _, interest := range interests1 {
        interestMap[interest] = true
    }
    
    matches := 0
    for _, interest := range interests2 {
        if interestMap[interest] {
            matches++
        }
    }
    
    // Jaccard similarity coefficient
    union := len(interests1) + len(interests2) - matches
    if union == 0 {
        return 0
    }
    
    return float64(matches) / float64(union)
}

func (m *matchingEngine) calculateLocationScore(lat1, lon1, lat2, lon2 float64) float64 {
    distance := m.haversineDistance(lat1, lon1, lat2, lon2)
    
    // Score based on distance (exponential decay)
    score := math.Exp(-distance / 50)
    
    return math.Min(1.0, math.Max(0, score))
}

func (m *matchingEngine) haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
    const earthRadius = 6371 // km
    
    dLat := (lat2 - lat1) * math.Pi / 180
    dLon := (lon2 - lon1) * math.Pi / 180
    
    a := math.Sin(dLat/2)*math.Sin(dLat/2) +
         math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
         math.Sin(dLon/2)*math.Sin(dLon/2)
    
    c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
    
    return earthRadius * c
}