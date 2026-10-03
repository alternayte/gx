package gx

import (
	"context"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/alternayte/gx/internal/ruleiter"
)

// Rule checks one input value (REQ-FRM-01). Every built-in rule leaves the
// zero value alone; Required rejects it.
type Rule struct {
	key      string
	check    func(v any) error
	checkCtx func(ctx context.Context, v any) error
}

// Field binds rules to one field. All rules must pass.
func Field[T any](v *T, rules ...Rule) Rule {
	return Rule{key: "field", check: func(any) error {
		val := any(*v)
		for _, r := range rules {
			if r.checkCtx != nil {
				continue
			}
			if err := r.check(val); err != nil {
				return ruleError(r.key, err)
			}
		}
		return nil
	}, checkCtx: func(ctx context.Context, _ any) error {
		val := any(*v)
		for _, r := range rules {
			if r.checkCtx != nil {
				if err := r.checkCtx(ctx, val); err != nil {
					return ruleError(r.key, err)
				}
				continue
			}
			if err := r.check(val); err != nil {
				return ruleError(r.key, err)
			}
		}
		return nil
	}}
}

// Rules is the rule set of an input type (REQ-FRM-01).
type Rules []Rule

// Unchecked marks an action input whose signal fields need no rules
// (SI-13). Embed it in the route struct.
type Unchecked struct{}

func violation(key, message string) error {
	return &FieldViolation{Key: key, Message: message}
}

// ruleError keeps a field violation and gives a plain error the rule key.
func ruleError(key string, err error) error {
	if _, ok := err.(*FieldViolation); ok {
		return err
	}
	return &FieldViolation{Key: key, Message: err.Error()}
}

// Required rejects the zero value of a string, number or bool.
var Required = Rule{key: "required", check: func(v any) error {
	switch x := v.(type) {
	case string:
		if x == "" {
			return violation("required", "required")
		}
	case bool:
		if !x {
			return violation("required", "required")
		}
	default:
		if n, ok := asInt64(v); ok && n == 0 {
			return violation("required", "required")
		}
		if f, ok := asFloat64(v); ok && f == 0 {
			return violation("required", "required")
		}
	}
	return nil
}}

// Email rejects a non-empty string that is not an email address.
var Email = Rule{key: "email", check: func(v any) error {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return violation("email", "must be a valid email address")
	}
	return nil
}}

// IsURL rejects a non-empty string that is not an http or https URL. It is
// named IsURL because gx.URL is the typed link value (REQ-RTE-05).
var IsURL = Rule{key: "url", check: func(v any) error {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return violation("url", "must be a valid URL")
	}
	return nil
}}

// MinLen rejects a string shorter than n code points.
func MinLen(n int) Rule {
	return Rule{key: "minlen", check: func(v any) error {
		s, ok := v.(string)
		if !ok {
			return nil
		}
		if utf8.RuneCountInString(s) < n {
			return violation("minlen", "must be at least "+strconv.Itoa(n)+" characters")
		}
		return nil
	}}
}

// MaxLen rejects a string longer than n code points.
func MaxLen(n int) Rule {
	return Rule{key: "maxlen", check: func(v any) error {
		s, ok := v.(string)
		if !ok {
			return nil
		}
		if utf8.RuneCountInString(s) > n {
			return violation("maxlen", "must be at most "+strconv.Itoa(n)+" characters")
		}
		return nil
	}}
}

// Min rejects a numeric value below n.
func Min(n int64) Rule {
	return Rule{key: "min", check: func(v any) error {
		if x, ok := asInt64(v); ok {
			if x < n {
				return violation("min", "must be at least "+strconv.FormatInt(n, 10))
			}
			return nil
		}
		if f, ok := asFloat64(v); ok && f < float64(n) {
			return violation("min", "must be at least "+strconv.FormatInt(n, 10))
		}
		return nil
	}}
}

// Max rejects a numeric value above n.
func Max(n int64) Rule {
	return Rule{key: "max", check: func(v any) error {
		if x, ok := asInt64(v); ok {
			if x > n {
				return violation("max", "must be at most "+strconv.FormatInt(n, 10))
			}
			return nil
		}
		if f, ok := asFloat64(v); ok && f > float64(n) {
			return violation("max", "must be at most "+strconv.FormatInt(n, 10))
		}
		return nil
	}}
}

// Pattern rejects a string that does not match a constant regexp.
func Pattern(re *regexp.Regexp) Rule {
	return Rule{key: "pattern", check: func(v any) error {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil
		}
		if !re.MatchString(s) {
			return violation("pattern", "must match "+re.String())
		}
		return nil
	}}
}

// OneOf rejects a string outside the allowed values.
func OneOf(values ...string) Rule {
	return Rule{key: "oneof", check: func(v any) error {
		s, ok := v.(string)
		if !ok {
			return nil
		}
		for _, want := range values {
			if s == want {
				return nil
			}
		}
		return violation("oneof", "must be one of "+joinQuoted(values))
	}}
}

// True rejects a false bool.
func True(key string) Rule {
	return Rule{key: key, check: func(v any) error {
		b, ok := v.(bool)
		if !ok || b {
			return nil
		}
		return violation(key, "must be accepted")
	}}
}

// Each applies a rule to every element of a slice or array.
func Each(rule Rule) Rule {
	return Rule{key: rule.key, check: func(v any) error {
		return ruleiter.Each(v, rule.check)
	}}
}

// Check runs a server-only function.
func Check(fn func(v any) error) Rule {
	return Rule{key: "check", check: fn}
}

// CheckCtx runs a server-only function with the request context.
func CheckCtx(fn func(ctx context.Context, v any) error) Rule {
	return Rule{key: "checkctx", checkCtx: fn}
}

// FieldViolation is one failed field rule (REQ-FRM-02, SI-13).
type FieldViolation struct {
	Field   string
	Key     string
	Message string
}

func (e *FieldViolation) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// RunRules runs the Rules of an input value and returns the first failure
// (SI-13).
func RunRules(v any) *FieldViolation {
	return RunRulesContext(context.Background(), v)
}

// RunRulesContext is RunRules with a request context for CheckCtx.
func RunRulesContext(ctx context.Context, v any) *FieldViolation {
	r, ok := v.(interface{ Rules() Rules })
	if !ok {
		return nil
	}
	for _, rule := range r.Rules() {
		var err error
		if rule.checkCtx != nil {
			err = rule.checkCtx(ctx, nil)
		} else {
			err = rule.check(nil)
		}
		if err != nil {
			if fv, ok := err.(*FieldViolation); ok {
				return fv
			}
			return &FieldViolation{Key: rule.key, Message: err.Error()}
		}
	}
	return nil
}

// asInt64 converts the integer kinds to int64.
func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		return int64(x), true
	}
	return 0, false
}

// asFloat64 converts the float kinds to float64.
func asFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// joinQuoted renders a value list for a message.
func joinQuoted(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		out += strconv.Quote(v)
	}
	return out
}
