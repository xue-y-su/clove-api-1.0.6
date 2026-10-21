package model

const (
	EmailTemplateKindBuiltin = "builtin"
	EmailTemplateKindCustom  = "custom"
)

// EmailTemplate stores verification email templates for the template center.
type EmailTemplate struct {
	ID          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	Kind        string `json:"kind" gorm:"type:varchar(16);not null;index"`
	Subject     string `json:"subject" gorm:"type:varchar(255);not null"`
	HTMLBody    string `json:"html_body" gorm:"type:text;not null"`
	IsDefault   bool   `json:"is_default" gorm:"not null;default:false;index"`
	Enabled     bool   `json:"enabled" gorm:"not null;default:true;index"`
	SortOrder   int    `json:"sort_order" gorm:"not null;default:0;index"`
	BuiltinKey  string `json:"builtin_key" gorm:"type:varchar(64);not null;default:'';index"`
	MigratedFrom string `json:"migrated_from" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func (EmailTemplate) TableName() string { return "email_templates" }

func GetDefaultEmailTemplate() (*EmailTemplate, error) {
	var tpl EmailTemplate
	err := DB.Where("is_default = ? AND enabled = ?", true, true).Order("id asc").First(&tpl).Error
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}

func GetEmailTemplateByID(id int64) (*EmailTemplate, error) {
	var tpl EmailTemplate
	if err := DB.First(&tpl, id).Error; err != nil {
		return nil, err
	}
	return &tpl, nil
}

func ListEmailTemplates() ([]EmailTemplate, error) {
	var items []EmailTemplate
	err := DB.Order("sort_order asc").Order("id asc").Find(&items).Error
	return items, err
}
