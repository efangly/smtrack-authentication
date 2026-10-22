package domain

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// Tenant is a customer organisation and the unit of tenancy.
type Tenant struct {
	ID          string
	Seq         int
	Name        string
	Kind        TenantKind
	Legacy      bool
	Hidden      bool
	Address     *string
	Tel         *string
	ContactName *string
	ContactTel  *string
	Latitude    *string
	Longitude   *string
	Pic         *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	DeletedBy   *string

	// Labels is the tenant's own label profile, as stored. Use EffectiveLabels
	// for what a client should display.
	Labels Labels
	// Departments is filled by reads that load them.
	Departments []Department
}

// EffectiveLabels is the label profile with the defaults for the tenant's kind
// filled in wherever the tenant has not set its own.
func (t *Tenant) EffectiveLabels() Labels {
	return t.Labels.WithDefaults(t.Kind)
}

// Department is a subdivision of a tenant where monitored devices live.
type Department struct {
	ID        string
	Seq       int
	TenantID  string
	Name      string
	Legacy    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	DeletedBy *string

	// Tenant is filled by reads that load it.
	Tenant *Tenant
}

// The label keys a tenant may rename.
const (
	LabelTenant     = "tenant"
	LabelDepartment = "department"
)

const maxLabelLength = 100

// Labels are a tenant's names for its concepts, per language:
// key (LabelTenant, LabelDepartment) -> language tag -> text.
type Labels map[string]map[string]string

// Validate checks a profile before it is stored: only known keys, at least a
// Thai or English text for each key that is set, and bounded text.
func (l Labels) Validate() error {
	for key, byLang := range l {
		if key != LabelTenant && key != LabelDepartment {
			return fmt.Errorf("unknown label %q", key)
		}
		if len(byLang) == 0 {
			continue
		}
		if byLang["th"] == "" && byLang["en"] == "" {
			return fmt.Errorf("label %q needs a th or en text", key)
		}
		for lang, text := range byLang {
			if lang == "" {
				return errors.New("label language must not be empty")
			}
			if text == "" {
				return fmt.Errorf("label %q (%s) must not be empty", key, lang)
			}
			if utf8.RuneCountInString(text) > maxLabelLength {
				return fmt.Errorf("label %q (%s) must be at most %d characters", key, lang, maxLabelLength)
			}
		}
	}
	return nil
}

// WithDefaults returns a copy of l where every key the tenant has not set
// takes the default for kind.
func (l Labels) WithDefaults(kind TenantKind) Labels {
	out := Labels{}
	for key, byLang := range defaultLabels(kind) {
		out[key] = byLang
	}
	for key, byLang := range l {
		if len(byLang) > 0 {
			out[key] = byLang
		}
	}
	return out
}

func defaultLabels(kind TenantKind) Labels {
	switch kind {
	case TenantKindLab:
		return Labels{
			LabelTenant:     {"th": "ห้องปฏิบัติการ", "en": "Laboratory"},
			LabelDepartment: {"th": "แผนก", "en": "Department"},
		}
	case TenantKindFactory:
		return Labels{
			LabelTenant:     {"th": "โรงงาน", "en": "Factory"},
			LabelDepartment: {"th": "แผนก", "en": "Department"},
		}
	case TenantKindClinic:
		return Labels{
			LabelTenant:     {"th": "คลินิก", "en": "Clinic"},
			LabelDepartment: {"th": "แผนก", "en": "Department"},
		}
	case TenantKindPharmacy:
		return Labels{
			LabelTenant:     {"th": "ร้านยา", "en": "Pharmacy"},
			LabelDepartment: {"th": "แผนก", "en": "Department"},
		}
	case TenantKindHospital:
		return Labels{
			LabelTenant:     {"th": "โรงพยาบาล", "en": "Hospital"},
			LabelDepartment: {"th": "วอร์ด", "en": "Ward"},
		}
	}
	return Labels{
		LabelTenant:     {"th": "องค์กร", "en": "Organisation"},
		LabelDepartment: {"th": "แผนก", "en": "Department"},
	}
}
