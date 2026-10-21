package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	emailTemplateMaxName    = 128
	emailTemplateMaxSubject = 255
	emailTemplateMaxBody    = 100_000
	builtinVerificationKey  = "verification_default"
)

var (
	ErrEmailTemplateInvalid   = errors.New("email template is invalid")
	ErrEmailTemplateNotFound  = errors.New("email template not found")
	ErrEmailTemplateForbidden = errors.New("email template action is not allowed")
	ErrEmailTemplateBuiltin   = errors.New("built-in email template cannot be modified that way")
)

var (
	disallowedHTMLTag = regexp.MustCompile(`(?is)<\s*(script|iframe|object|embed|form|link|meta|base|svg|math)[^>]*>`)
	disallowedAttr    = regexp.MustCompile(`(?is)\s(on\w+|style)\s*=`)
	jsURLAttr         = regexp.MustCompile(`(?is)\s(href|src)\s*=\s*['"]?\s*javascript:`)
)

type EmailTemplateInput struct {
	Name     string
	Subject  string
	HTMLBody string
	Enabled  *bool
}

type EmailTemplatePreview struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type EmailTemplateTestSendInput struct {
	TemplateID int64
	To         string
	ActorID    int
}

func EnsureBuiltinEmailTemplates(ctx context.Context) error {
	if model.DB == nil {
		return ErrEmailTemplateInvalid
	}
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.EmailTemplate{}).Where("builtin_key = ?", builtinVerificationKey).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			tpl := model.EmailTemplate{
				Name:       "默认邮箱验证模板",
				Kind:       model.EmailTemplateKindBuiltin,
				Subject:    "{{site_name}} 邮箱验证",
				HTMLBody:   defaultVerificationHTML(),
				IsDefault:  true,
				Enabled:    true,
				SortOrder:  0,
				BuiltinKey: builtinVerificationKey,
			}
			if err := tx.Create(&tpl).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&model.EmailTemplate{}).
			Where("builtin_key = ?", builtinVerificationKey).
			Updates(map[string]any{
				"name":      "默认邮箱验证模板",
				"subject":   "{{site_name}} 邮箱验证",
				"html_body": defaultVerificationHTML(),
			}).Error; err != nil {
			return err
		}
		// One-time migration from legacy option value when present and no custom migrated row.
		legacy := strings.TrimSpace(common.EmailVerificationTemplate)
		if legacy == "" {
			return nil
		}
		var migrated int64
		if err := tx.Model(&model.EmailTemplate{}).Where("migrated_from = ?", "EmailVerificationTemplate").Count(&migrated).Error; err != nil {
			return err
		}
		if migrated > 0 {
			return tx.Model(&model.EmailTemplate{}).
				Where("migrated_from = ?", "EmailVerificationTemplate").
				Updates(map[string]any{
					"name":    "从旧设置迁移的邮箱验证模板",
					"subject": "{{site_name}} 邮箱验证",
				}).Error
		}
		if err := validateTemplateHTML(legacy); err != nil {
			// Leave invalid legacy content in Option; do not create a broken template.
			return nil
		}
		tpl := model.EmailTemplate{
			Name:         "从旧设置迁移的邮箱验证模板",
			Kind:         model.EmailTemplateKindCustom,
			Subject:      "{{site_name}} 邮箱验证",
			HTMLBody:     legacy,
			IsDefault:    false,
			Enabled:      true,
			SortOrder:    10,
			MigratedFrom: "EmailVerificationTemplate",
		}
		return tx.Create(&tpl).Error
	})
}

func ListEmailTemplates(ctx context.Context) ([]model.EmailTemplate, error) {
	if model.DB == nil {
		return nil, ErrEmailTemplateInvalid
	}
	if err := EnsureBuiltinEmailTemplates(ctx); err != nil {
		return nil, err
	}
	return model.ListEmailTemplates()
}

func CreateEmailTemplate(ctx context.Context, input EmailTemplateInput) (*model.EmailTemplate, error) {
	if err := validateEmailTemplateInput(input); err != nil {
		return nil, err
	}
	tpl := model.EmailTemplate{
		Name:      strings.TrimSpace(input.Name),
		Kind:      model.EmailTemplateKindCustom,
		Subject:   strings.TrimSpace(input.Subject),
		HTMLBody:  input.HTMLBody,
		Enabled:   true,
		SortOrder: 100,
	}
	if input.Enabled != nil {
		tpl.Enabled = *input.Enabled
	}
	if err := model.DB.WithContext(ctx).Create(&tpl).Error; err != nil {
		return nil, err
	}
	return &tpl, nil
}

func UpdateEmailTemplate(ctx context.Context, id int64, input EmailTemplateInput) (*model.EmailTemplate, error) {
	if id <= 0 {
		return nil, ErrEmailTemplateInvalid
	}
	if err := validateEmailTemplateInput(input); err != nil {
		return nil, err
	}
	var tpl model.EmailTemplate
	if err := model.DB.WithContext(ctx).First(&tpl, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEmailTemplateNotFound
		}
		return nil, err
	}
	if tpl.Kind == model.EmailTemplateKindBuiltin {
		return nil, ErrEmailTemplateBuiltin
	}
	updates := map[string]any{
		"name":      strings.TrimSpace(input.Name),
		"subject":   strings.TrimSpace(input.Subject),
		"html_body": input.HTMLBody,
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}
	if err := model.DB.WithContext(ctx).Model(&tpl).Updates(updates).Error; err != nil {
		return nil, err
	}
	return model.GetEmailTemplateByID(id)
}

func DeleteEmailTemplate(ctx context.Context, id int64) error {
	var tpl model.EmailTemplate
	if err := model.DB.WithContext(ctx).First(&tpl, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailTemplateNotFound
		}
		return err
	}
	if tpl.Kind == model.EmailTemplateKindBuiltin {
		return ErrEmailTemplateBuiltin
	}
	if tpl.IsDefault {
		return ErrEmailTemplateInvalid
	}
	return model.DB.WithContext(ctx).Delete(&tpl).Error
}

func SetDefaultEmailTemplate(ctx context.Context, id int64) error {
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tpl model.EmailTemplate
		if err := tx.First(&tpl, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEmailTemplateNotFound
			}
			return err
		}
		if !tpl.Enabled {
			return ErrEmailTemplateInvalid
		}
		if err := tx.Model(&model.EmailTemplate{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&tpl).Update("is_default", true).Error
	})
}

func PreviewEmailTemplate(ctx context.Context, id int64) (*EmailTemplatePreview, error) {
	tpl, err := resolveTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	subject, body := renderEmailTemplate(tpl.Subject, tpl.HTMLBody, sampleTemplateValues())
	return &EmailTemplatePreview{Subject: subject, HTML: body}, nil
}

func RenderActiveVerificationEmail(code string) (subject string, htmlBody string, err error) {
	ctx := context.Background()
	if err := EnsureBuiltinEmailTemplates(ctx); err != nil {
		// Fall back to legacy path without failing registration.
		return "", "", err
	}
	tpl, err := model.GetDefaultEmailTemplate()
	if err != nil {
		// Compatibility: legacy option template when no default row.
		legacy := strings.TrimSpace(common.EmailVerificationTemplate)
		if legacy != "" {
			subject = fmt.Sprintf("%s 邮箱验证", common.SystemName)
			htmlBody = renderPlaceholders(legacy, map[string]string{
				"code":               code,
				"site_name":          common.SystemName,
				"expiration_minutes": fmt.Sprintf("%d", common.VerificationValidMinutes),
			})
			return subject, htmlBody, nil
		}
		return "", "", err
	}
	subject, htmlBody = renderEmailTemplate(tpl.Subject, tpl.HTMLBody, map[string]string{
		"code":               code,
		"site_name":          common.SystemName,
		"expiration_minutes": fmt.Sprintf("%d", common.VerificationValidMinutes),
	})
	return subject, htmlBody, nil
}

func TestSendEmailTemplate(ctx context.Context, input EmailTemplateTestSendInput) error {
	to := strings.TrimSpace(input.To)
	if to == "" || input.TemplateID <= 0 {
		return ErrEmailTemplateInvalid
	}
	preview, err := PreviewEmailTemplate(ctx, input.TemplateID)
	if err != nil {
		return err
	}
	return common.SendEmail(preview.Subject, to, preview.HTML)
}

func resolveTemplate(ctx context.Context, id int64) (*model.EmailTemplate, error) {
	if model.DB == nil || id <= 0 {
		return nil, ErrEmailTemplateInvalid
	}
	tpl, err := model.GetEmailTemplateByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEmailTemplateNotFound
		}
		return nil, err
	}
	return tpl, nil
}

func validateEmailTemplateInput(input EmailTemplateInput) error {
	name := strings.TrimSpace(input.Name)
	subject := strings.TrimSpace(input.Subject)
	if name == "" || utf8.RuneCountInString(name) > emailTemplateMaxName {
		return ErrEmailTemplateInvalid
	}
	if subject == "" || utf8.RuneCountInString(subject) > emailTemplateMaxSubject {
		return ErrEmailTemplateInvalid
	}
	if strings.TrimSpace(input.HTMLBody) == "" || len(input.HTMLBody) > emailTemplateMaxBody {
		return ErrEmailTemplateInvalid
	}
	return validateTemplateHTML(input.HTMLBody)
}

func validateTemplateHTML(body string) error {
	if disallowedHTMLTag.MatchString(body) || disallowedAttr.MatchString(body) || jsURLAttr.MatchString(body) {
		return ErrEmailTemplateInvalid
	}
	return nil
}

func defaultVerificationHTML() string {
	return `<p>您好，您正在验证 {{site_name}} 的邮箱。</p>
<p>您的邮箱验证码是：<strong>{{code}}</strong></p>
<p>验证码有效期为 {{expiration_minutes}} 分钟。如非本人操作，请忽略本邮件。</p>`
}

func sampleTemplateValues() map[string]string {
	return map[string]string{
		"code":               "123456",
		"site_name":          common.SystemName,
		"expiration_minutes": fmt.Sprintf("%d", common.VerificationValidMinutes),
	}
}

func renderEmailTemplate(subject, body string, values map[string]string) (string, string) {
	return renderPlaceholders(subject, values), renderPlaceholders(body, values)
}

func renderPlaceholders(template string, values map[string]string) string {
	out := template
	for k, v := range values {
		out = strings.ReplaceAll(out, "{{"+k+"}}", html.EscapeString(v))
	}
	return out
}
