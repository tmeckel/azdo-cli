package shared

import (
	"testing"
)

func TestNormalizeClassificationPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", ""},
		{"spaces only", "   ", ""},
		{"backslashes", `\Project\Area\Foo\`, "Project/Area/Foo"},
		{"leading/trailing slashes", "/a/b/c/", "a/b/c"},
		{"consecutive slashes", "a//b///c", "a/b/c"},
		{"mixed separators and spaces", `  \A//B\C/  `, "A/B/C"},
		{"only slashes", "///\\\\///", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeClassificationPath(tt.in)
			if got != tt.out {
				t.Fatalf("NormalizeClassificationPath(%q) = %q, want %q", tt.in, got, tt.out)
			}
		})
	}
}

func TestBuildClassificationPath(t *testing.T) {
	type args struct {
		project       string
		includesScope bool
		scopeName     string
		raw           string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"empty returns empty", args{
				"proj", false, "", "",
			}, "", false,
		},
		{"only slashes returns empty", args{
			"proj", false, "", "////",
		}, "", false},
		{"simple path", args{
			"proj", false, "", "area/sub",
		}, "area/sub", false},
		{"backslashes and spaces", args{
			"proj", false, "", `\Area \Sub `,
		}, "Area/Sub", false},
		{"collapse slashes", args{
			"proj", false, "", "a//b///c",
		}, "a/b/c", false},
		{"leading project removed", args{
			"MyProj", false, "", "MyProj/Area/Child",
		}, "Area/Child", false},
		{"leading project removed case-insensitive", args{
			"myproj", false, "", "MYPROJ/Area",
		}, "Area", false},
		{"remove scope when requested", args{
			"proj", true, "Area", "proj/Area/Sub",
		}, "Sub", false},
		{"remove scope case-insensitive", args{
			"proj", true, "Area", "proj/area/Sub",
		}, "Sub", false},
		{"segment with reserved chars escaped", args{
			"proj", false, "", "a/b c/d?e",
		}, "a/b%20c/d%3Fe", false},
		// New test: handle space-only segments by skipping them.
		{"skip space-only segments and return last meaningful segment", args{
			"SP4DB", true, "Area", "SP4DB/Area/ /   /WKB",
		}, "WKB", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildClassificationPath(tt.args.project, tt.args.includesScope, tt.args.scopeName, tt.args.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("BuildClassificationPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("BuildClassificationPath() = %q, want %q (err=%v)", got, tt.want, err)
			}
		})
	}
}
