package catalog

import (
	"strings"
	"unicode"
)

type normalizedMedicineIdentity struct {
	GenericName string
	BrandName   *string
	Form        string
	Strength    *string
}

func normalizeCatalogText(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(trimmed))

	prevSpace := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			if !prevSpace && b.Len() > 0 {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			if !prevSpace && b.Len() > 0 {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func normalizeCatalogPtr(input *string) *string {
	if input == nil {
		return nil
	}
	normalized := normalizeCatalogText(*input)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func normalizedValue(input *string) string {
	if input == nil {
		return ""
	}
	return *input
}

func buildNormalizedMedicineIdentity(input MedicineImportPayload) normalizedMedicineIdentity {
	return normalizedMedicineIdentity{
		GenericName: normalizeCatalogText(input.GenericName),
		BrandName:   normalizeCatalogPtr(input.BrandName),
		Form:        normalizeCatalogText(input.Form),
		Strength:    normalizeCatalogPtr(input.Strength),
	}
}

func (n normalizedMedicineIdentity) Response() NormalizedMedicineInput {
	return NormalizedMedicineInput{
		GenericName: n.GenericName,
		BrandName:   n.BrandName,
		Form:        n.Form,
		Strength:    n.Strength,
	}
}
