package gx_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/alternayte/gx"
)

// probeRules makes a rules set runnable without a generated form type.
type probeRules struct{ rules gx.Rules }

func (p *probeRules) Rules() gx.Rules { return p.rules }

// TestREQ_FRM_01_Rules is the table test per built-in rule.
func TestREQ_FRM_01_Rules(t *testing.T) {
	cases := []struct {
		name string
		run  func() *gx.FieldViolation
		key  string
	}{
		{"required empty", func() *gx.FieldViolation {
			s := ""
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.Required)}})
		}, "required"},
		{"required set", func() *gx.FieldViolation {
			s := "x"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.Required)}})
		}, ""},
		{"email bad", func() *gx.FieldViolation {
			s := "not an email"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.Email)}})
		}, "email"},
		{"email good", func() *gx.FieldViolation {
			s := "a@b.co"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.Email)}})
		}, ""},
		{"url bad", func() *gx.FieldViolation {
			s := "javascript:alert(1)"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.IsURL)}})
		}, "url"},
		{"url good", func() *gx.FieldViolation {
			s := "https://example.com/x"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.IsURL)}})
		}, ""},
		{"minlen", func() *gx.FieldViolation {
			s := "ab"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.MinLen(3))}})
		}, "minlen"},
		{"maxlen", func() *gx.FieldViolation {
			s := "abcd"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.MaxLen(3))}})
		}, "maxlen"},
		{"maxlen runes", func() *gx.FieldViolation {
			s := "héllo"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.MaxLen(5))}})
		}, ""},
		{"min", func() *gx.FieldViolation {
			n := 0
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&n, gx.Min(1))}})
		}, "min"},
		{"max", func() *gx.FieldViolation {
			n := 9
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&n, gx.Max(5))}})
		}, "max"},
		{"max float", func() *gx.FieldViolation {
			f := 5.5
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&f, gx.Max(5))}})
		}, "max"},
		{"pattern", func() *gx.FieldViolation {
			s := "abc"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.Pattern(regexp.MustCompile(`^[0-9]+$`)))}})
		}, "pattern"},
		{"oneof", func() *gx.FieldViolation {
			s := "c"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.OneOf("a", "b"))}})
		}, "oneof"},
		{"true", func() *gx.FieldViolation {
			b := false
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&b, gx.True("terms.required"))}})
		}, "terms.required"},
		{"each", func() *gx.FieldViolation {
			xs := []int{3, 0}
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&xs, gx.Each(gx.Min(1)))}})
		}, "min"},
		{"check", func() *gx.FieldViolation {
			s := "taken"
			return gx.RunRules(&probeRules{gx.Rules{gx.Field(&s, gx.Check(func(v any) error {
				if v == "taken" {
					return errors.New("taken")
				}
				return nil
			}))}})
		}, "check"},
		{"checkctx", func() *gx.FieldViolation {
			s := "x"
			return gx.RunRulesContext(context.Background(), &probeRules{gx.Rules{gx.Field(&s, gx.CheckCtx(func(ctx context.Context, v any) error {
				if ctx == nil {
					return errors.New("no context")
				}
				return nil
			}))}})
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.run()
			if c.key == "" {
				if got != nil {
					t.Fatalf("unexpected violation %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("no violation, want key %q", c.key)
			}
			if got.Key != c.key {
				t.Fatalf("key = %q, want %q", got.Key, c.key)
			}
		})
	}
}
