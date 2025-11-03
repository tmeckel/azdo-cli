package util

import "testing"

func TestIsSecurityIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "valid sid uppercase",
			value: "S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1",
			want:  true,
		},
		{
			name:  "valid sid lowercase",
			value: "s-1-5-21-123456789-123456789-123456789-1000",
			want:  true,
		},
		{
			name:  "not sid descriptor with dot",
			value: "vssgp.Uy0xLTktMTIz",
			want:  false,
		},
		{
			name:  "empty string",
			value: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSecurityIdentifier(tt.value); got != tt.want {
				t.Fatalf("IsSecurityIdentifier(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsDescriptorNotRecognizesSID(t *testing.T) {
	sid := "S-1-5-21-123456789-123456789-123456789-1000"
	if IsDescriptor(sid) {
		t.Fatalf("expected SID %q to be treated as descriptor", sid)
	}
}
