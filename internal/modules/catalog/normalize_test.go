package catalog

import "testing"

func TestNormalizeCatalogText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trim lower and collapse whitespace",
			input: "  Paracetamol   500MG  ",
			want:  "paracetamol 500mg",
		},
		{
			name:  "replace punctuation with spaces",
			input: "Ibuprofen-Soft.Gel",
			want:  "ibuprofen soft gel",
		},
		{
			name:  "keep unicode letters and digits",
			input: "Амоксициллин 250мг",
			want:  "амоксициллин 250мг",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeCatalogText(tt.input); got != tt.want {
				t.Fatalf("normalizeCatalogText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShouldSuggestSingleMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		candidates []MedicineSuggestion
		want       bool
	}{
		{
			name:       "single candidate suggests",
			candidates: []MedicineSuggestion{{Score: 0.61}},
			want:       true,
		},
		{
			name: "high confidence gap suggests",
			candidates: []MedicineSuggestion{
				{Score: 0.91},
				{Score: 0.74},
			},
			want: true,
		},
		{
			name: "close scores stay ambiguous",
			candidates: []MedicineSuggestion{
				{Score: 0.83},
				{Score: 0.78},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldSuggestSingleMatch(tt.candidates); got != tt.want {
				t.Fatalf("shouldSuggestSingleMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
