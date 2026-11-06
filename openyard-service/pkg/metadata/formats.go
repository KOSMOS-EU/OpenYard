package metadata

import (
	"regexp"
	"strings"
	"time"
)

// FormatConfig converts values between the canonical storage format
// (ISO date 2006-01-02, decimal point) and the client's format.
// Saskia reads and writes German dates and decimal commas and cannot be
// configured otherwise; Taki, search and smart folders use the canonical form.
type FormatConfig struct {
	// Date is the client's date layout in Go notation, e.g. "02.01.2006".
	// Empty: dates are passed unchanged.
	Date string `yaml:"date"`

	// Decimal is the client's decimal separator, e.g. ",". Applies to
	// amounts.* only (other numbers like versions stay untouched).
	// Empty: numbers are passed unchanged.
	Decimal string `yaml:"decimal"`
}

const isoDateLayout = "2006-01-02"

var (
	isoDatePattern       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	pointDecimalPattern  = regexp.MustCompile(`^-?\d+\.\d+$`)
	clientDecimalPattern = regexp.MustCompile(`^-?\d{1,3}(\.\d{3})*,\d+$|^-?\d+,\d+$`)
)

// isAmountKey reports whether numbers of this storage key are converted.
func isAmountKey(storageKey string) bool {
	return strings.HasPrefix(storageKey, "amounts.")
}

// ToClientValue converts a stored value into the format of login's namespace.
func (c *Config) ToClientValue(login, storageKey, value string) string {
	formats := c.NamespaceForUser(login).Formats
	if formats.Date != "" && isoDatePattern.MatchString(value) {
		if parsed, err := time.Parse(isoDateLayout, value); err == nil {
			return parsed.Format(formats.Date)
		}
	}
	if formats.Decimal != "" && formats.Decimal != "." && isAmountKey(storageKey) && pointDecimalPattern.MatchString(value) {
		return strings.Replace(value, ".", formats.Decimal, 1)
	}
	return value
}

// ToStorageValue converts a value sent by login's client into the canonical
// storage format. Values in other shapes pass unchanged.
func (c *Config) ToStorageValue(login, storageKey, value string) string {
	formats := c.NamespaceForUser(login).Formats
	trimmed := strings.TrimSpace(value)
	if formats.Date != "" && len(trimmed) == len(formats.Date) {
		if parsed, err := time.Parse(formats.Date, trimmed); err == nil {
			return parsed.Format(isoDateLayout)
		}
	}
	if formats.Decimal == "," && isAmountKey(storageKey) && clientDecimalPattern.MatchString(trimmed) {
		withoutThousands := strings.ReplaceAll(trimmed, ".", "")
		return strings.Replace(withoutThousands, ",", ".", 1)
	}
	return value
}
