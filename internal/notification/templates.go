// internal/notification/templates.go

package notifications

import (
    "bytes"
    "context"
    "fmt"
    "text/template"
)

// DefaultTemplateService provides template rendering for notifications
type DefaultTemplateService struct {
    repo Repository
}

// NewTemplateService creates a new template service
func NewTemplateService(repo Repository) TemplateService {
    return &DefaultTemplateService{repo: repo}
}

// RenderTemplate renders a notification template with variables
func (s *DefaultTemplateService) RenderTemplate(ctx context.Context, templateType NotificationType, language string, data map[string]interface{}) (title, body string, err error) {
    // Get template from database
    tmpl, err := s.repo.GetTemplate(ctx, templateType, language)
    if err != nil {
        // Fall back to default templates
        return s.getDefaultTemplate(templateType, data)
    }
    
    // Render title
    title, err = s.renderString(tmpl.TitleTemplate, data)
    if err != nil {
        return "", "", fmt.Errorf("failed to render title: %v", err)
    }
    
    // Render body
    body, err = s.renderString(tmpl.BodyTemplate, data)
    if err != nil {
        return "", "", fmt.Errorf("failed to render body: %v", err)
    }
    
    return title, body, nil
}

// renderString renders a template string with data
func (s *DefaultTemplateService) renderString(templateStr string, data map[string]interface{}) (string, error) {
    tmpl, err := template.New("notification").Parse(templateStr)
    if err != nil {
        return "", err
    }
    
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    
    return buf.String(), nil
}

// getDefaultTemplate returns default templates for notification types
func (s *DefaultTemplateService) getDefaultTemplate(notificationType NotificationType, data map[string]interface{}) (title, body string, err error) {
    switch notificationType {
    case TypeWelcome:
        title = "Welcome to Kiekky! 🎉"
        body = "Start exploring and connecting with amazing people around you."
        
    case TypeFollow:
        username := s.getStringValue(data, "username", "Someone")
        title = "New Follower! 👥"
        body = fmt.Sprintf("%s started following you", username)
        
    case TypeLike:
        username := s.getStringValue(data, "username", "Someone")
        title = "Someone liked your post! ❤️"
        body = fmt.Sprintf("%s liked your post", username)
        
    case TypeComment:
        username := s.getStringValue(data, "username", "Someone")
        comment := s.getStringValue(data, "comment", "")
        title = "New comment on your post 💬"
        body = fmt.Sprintf("%s commented: %s", username, s.truncate(comment, 50))
        
    case TypeMessage:
        username := s.getStringValue(data, "username", "Someone")
        message := s.getStringValue(data, "message", "")
        title = fmt.Sprintf("Message from %s 💌", username)
        body = s.truncate(message, 100)
        
    case TypeMatch:
        username := s.getStringValue(data, "username", "someone")
        title = "It's a Match! 💕"
        body = fmt.Sprintf("You matched with %s! Start a conversation now.", username)
        
    case TypeStoryView:
        username := s.getStringValue(data, "username", "Someone")
        title = "Story View 👀"
        body = fmt.Sprintf("%s viewed your story", username)
        
    case TypeStoryReply:
        username := s.getStringValue(data, "username", "Someone")
        reply := s.getStringValue(data, "reply", "")
        title = "Story Reply 💬"
        body = fmt.Sprintf("%s replied to your story: %s", username, s.truncate(reply, 50))
        
    case TypeMention:
        username := s.getStringValue(data, "username", "Someone")
        title = "You were mentioned! 📢"
        body = fmt.Sprintf("%s mentioned you in a post", username)
        
    case TypeVerification:
        code := s.getStringValue(data, "code", "")
        title = "Verify Your Account 🔐"
        body = fmt.Sprintf("Your verification code is: %s", code)
        
    case TypeSecurity:
        action := s.getStringValue(data, "action", "security alert")
        title = "Security Alert 🚨"
        body = fmt.Sprintf("We noticed %s on your account", action)
        
    case TypePromotion:
        offer := s.getStringValue(data, "offer", "special offer")
        title = "Special Offer! 🎁"
        body = fmt.Sprintf("Check out our %s", offer)
        
    case TypeMaintenance:
        time := s.getStringValue(data, "time", "soon")
        title = "Scheduled Maintenance 🔧"
        body = fmt.Sprintf("We'll be performing maintenance %s", time)
        
    default:
        title = "Kiekky Notification"
        body = "You have a new notification"
    }
    
    return title, body, nil
}

// getStringValue safely gets a string value from map
func (s *DefaultTemplateService) getStringValue(data map[string]interface{}, key, defaultValue string) string {
    if val, ok := data[key]; ok {
        if strVal, ok := val.(string); ok {
            return strVal
        }
    }
    return defaultValue
}

// truncate truncates a string to specified length
func (s *DefaultTemplateService) truncate(temp string, maxLen int) string {
    if len(temp) <= maxLen {
        return temp
    }
    return temp[:maxLen-3] + "..."
}

// Default notification templates for different languages
var defaultTemplates = map[string]map[NotificationType]*NotificationTemplate{
    "en": {
        TypeWelcome: {
            Type:          TypeWelcome,
            Language:      "en",
            TitleTemplate: "Welcome to Kiekky, {{.username}}! 🎉",
            BodyTemplate:  "We're excited to have you join our community. Complete your profile to get started!",
            Variables:     []string{"username"},
        },
        TypeFollow: {
            Type:          TypeFollow,
            Language:      "en",
            TitleTemplate: "New Follower! 👥",
            BodyTemplate:  "{{.follower_name}} started following you",
            Variables:     []string{"follower_name", "follower_id"},
        },
        TypeLike: {
            Type:          TypeLike,
            Language:      "en",
            TitleTemplate: "Your post got a new like! ❤️",
            BodyTemplate:  "{{.liker_name}} liked your post",
            Variables:     []string{"liker_name", "liker_id", "post_id"},
        },
        TypeComment: {
            Type:          TypeComment,
            Language:      "en",
            TitleTemplate: "New comment on your post 💬",
            BodyTemplate:  "{{.commenter_name}}: {{.comment}}",
            Variables:     []string{"commenter_name", "commenter_id", "post_id", "comment"},
        },
        TypeMessage: {
            Type:          TypeMessage,
            Language:      "en",
            TitleTemplate: "Message from {{.sender_name}} 💌",
            BodyTemplate:  "{{.message}}",
            Variables:     []string{"sender_name", "sender_id", "message"},
        },
        TypeMatch: {
            Type:          TypeMatch,
            Language:      "en",
            TitleTemplate: "It's a Match! 💕",
            BodyTemplate:  "You matched with {{.matched_user_name}}! Start a conversation now.",
            Variables:     []string{"matched_user_name", "matched_user_id"},
        },
    },
    "fr": {
        TypeWelcome: {
            Type:          TypeWelcome,
            Language:      "fr",
            TitleTemplate: "Bienvenue sur Kiekky, {{.username}}! 🎉",
            BodyTemplate:  "Nous sommes ravis de vous accueillir dans notre communauté. Complétez votre profil pour commencer!",
            Variables:     []string{"username"},
        },
        // Add more French templates...
    },
    "es": {
        TypeWelcome: {
            Type:          TypeWelcome,
            Language:      "es",
            TitleTemplate: "¡Bienvenido a Kiekky, {{.username}}! 🎉",
            BodyTemplate:  "Estamos emocionados de tenerte en nuestra comunidad. ¡Completa tu perfil para empezar!",
            Variables:     []string{"username"},
        },
        // Add more Spanish templates...
    },
}

// InitializeDefaultTemplates initializes default templates in the database
func InitializeDefaultTemplates(ctx context.Context, repo Repository) error {
    for lang, templates := range defaultTemplates {
        for _, tmpl := range templates {
            existing, err := repo.GetTemplate(ctx, tmpl.Type, lang)
            if err != nil || existing == nil {
                if err := repo.CreateTemplate(ctx, tmpl); err != nil {
                    return fmt.Errorf("failed to create template for %s in %s: %v", tmpl.Type, lang, err)
                }
            }
        }
    }
    
    return nil
}

// MockTemplateService is a mock implementation for testing
type MockTemplateService struct{}

func NewMockTemplateService() TemplateService {
    return &MockTemplateService{}
}

func (m *MockTemplateService) RenderTemplate(ctx context.Context, templateType NotificationType, language string, data map[string]interface{}) (title, body string, err error) {
    return fmt.Sprintf("Test %s", templateType), "Test notification body", nil
}