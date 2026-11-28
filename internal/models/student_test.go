package models

import (
	"reflect"
	"testing"
)

func TestParseStatusTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "empty string",
			raw:  "",
			want: nil,
		},
		{
			name: "whitespace and commas only",
			raw:  " , ,  , ",
			want: nil,
		},
		{
			name: "trims whitespace and drops empty segments",
			raw:  " Active , Varsity, , On Hold ",
			want: []string{"Active", "Varsity", "On Hold"},
		},
		{
			name: "deduplicates case-insensitively preserving first casing",
			raw:  "Varsity, varsity, Competition Team, VARSITY, Trial",
			want: []string{"Varsity", "Competition Team", "Trial"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseStatusTags(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseStatusTags(%q) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestFormatStatusTags(t *testing.T) {
	if got := FormatStatusTags(nil); got != "" {
		t.Fatalf("FormatStatusTags(nil) = %q, want empty", got)
	}

	got := FormatStatusTags([]string{" Active ", "active", "Varsity", "", "Scholarship"})
	want := "Active,Varsity,Scholarship"
	if got != want {
		t.Fatalf("FormatStatusTags(...) = %q, want %q", got, want)
	}
}
