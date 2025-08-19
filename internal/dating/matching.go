// internal/dating/matching.go

package dating

import (
    "context"
    "math"
    "sort"
    "fmt"
    "time"
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

func (m *matchingEngine) GenerateRecommendations(ctx context.Context, userProfile *UserProfile, candidates []*UserProfile) ([]*ScoredCandidate, error) {
    scored := make([]*ScoredCandidate, 0, len(candidates))
    
    for _, candidate := range candidates {
        score, factors, err := m.CalculateCompatibility(ctx, userProfile, candidate)
        if err != nil {
            continue
        }
        
        reason := m.generateReasonForMatch(factors, candidate)
        
        scored = append(scored, &ScoredCandidate{
            UserID:  candidate.ID,
            Profile: candidate,
            Score:   score,
            Factors: factors,
            Reason:  reason,
        })
    }
    
    // Sort by score descending
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })
    
    return scored, nil
}

func (m *matchingEngine) UpdateUserFactors(ctx context.Context, userID int64, interactions []*UserInteraction) error {
    // This would update user preferences based on their interaction patterns
    // For example, if they consistently like profiles with certain interests,
    // we would weight those interests higher in future recommendations
    
    // Implementation would involve:
    // 1. Analyzing interaction patterns
    // 2. Updating user preference weights
    // 3. Storing updated factors in database
    
    return nil
}

func (m *matchingEngine) generateReasonForMatch(factors *CompatibilityFactors, candidate *UserProfile) string {
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
    if factors.AgeCompatibility > 0.9 {
        reasons = append(reasons, "perfect age match")
    }
    
    if len(reasons) == 0 {
        return "Recommended for you"
    }
    
    return fmt.Sprintf("%s %s", candidate.DisplayName, reasons[0])
}

func (m *matchingEngine) calculateAgeScore(age1, age2 int) float64 {
    ageDiff := math.Abs(float64(age1 - age2))
    
    // Score based on age difference
    // 0-2 years: perfect match (1.0)
    // 3-5 years: good match (0.8)
    // 6-10 years: okay match (0.5)
    // >10 years: lower scores
    
    switch {
    case ageDiff <= 2:
        return 1.0
    case ageDiff <= 5:
        return 0.8
    case ageDiff <= 10:
        return 0.5
    default:
        return math.Max(0, 1.0 - (ageDiff-10)/20)
    }
}

func (m *matchingEngine) calculateActivityScore(lastActive1, lastActive2 time.Time, pattern1, pattern2 *string) float64 {
    // Calculate based on activity patterns
    daysSinceActive1 := time.Since(lastActive1).Hours() / 24
    daysSinceActive2 := time.Since(lastActive2).Hours() / 24
    
    // Both recently active
    if daysSinceActive1 < 7 && daysSinceActive2 < 7 {
        return 0.9
    }
    
    // One recently active
    if daysSinceActive1 < 7 || daysSinceActive2 < 7 {
        return 0.6
    }
    
    // Both somewhat active
    if daysSinceActive1 < 30 && daysSinceActive2 < 30 {
        return 0.4
    }
    
    return 0.2
}

func (m *matchingEngine) calculatePreferencesScore(looking1, looking2, gender1, gender2 string) float64 {
    // Check if both are looking for the same type of relationship
    if looking1 == looking2 {
        return 1.0
    }
    
    // Compatible relationship types
    compatibleTypes := map[string][]string{
        "relationship": {"relationship", "dating"},
        "dating":       {"relationship", "dating", "casual"},
        "casual":       {"casual", "dating"},
        "friendship":   {"friendship"},
    }
    
    if compatible, ok := compatibleTypes[looking1]; ok {
        for _, t := range compatible {
            if t == looking2 {
                return 0.7
            }
        }
    }
    
    return 0.3
}

func (m *matchingEngine) calculateEngagementScore(responseRate1, responseRate2 float64, activeDays1, activeDays2 int) float64 {
    avgResponseRate := (responseRate1 + responseRate2) / 2
    avgActiveDays := float64(activeDays1+activeDays2) / 2
    
    // Weight response rate more heavily
    score := (avgResponseRate * 0.7) + (math.Min(avgActiveDays/30, 1.0) * 0.3)
    
    return score
}