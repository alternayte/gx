package gx

import "strconv"

// Rule checks one input value (REQ-FRM-01, SI-13).
type Rule struct {
	key   string
	check func(v any) error
}

// Field binds rules to one field. All rules must pass.
func Field[T any](v *T, rules ...Rule) Rule {
	return Rule{key: "field", check: func(any) error {
		for _, r := range rules {
			if err := r.check(any(*v)); err != nil {
				return err
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

// Required rejects the zero value of a string, number or bool.
var Required = Rule{key: "required", check: func(v any) error {
	switch x := v.(type) {
	case string:
		if x == "" {
			return &FieldViolation{Key: "required", Message: "required"}
		}
	case int:
		if x == 0 {
			return &FieldViolation{Key: "required", Message: "required"}
		}
	case int64:
		if x == 0 {
			return &FieldViolation{Key: "required", Message: "required"}
		}
	case float64:
		if x == 0 {
			return &FieldViolation{Key: "required", Message: "required"}
		}
	case bool:
		if !x {
			return &FieldViolation{Key: "required", Message: "required"}
		}
	}
	return nil
}}

// Min rejects a numeric value below n.
func Min(n int64) Rule {
	return Rule{key: "min", check: func(v any) error {
		if x, ok := asInt64(v); ok && x < n {
			return &FieldViolation{Key: "min", Message: "must be at least " + strconv.FormatInt(n, 10)}
		}
		return nil
	}}
}

// Max rejects a numeric value above n.
func Max(n int64) Rule {
	return Rule{key: "max", check: func(v any) error {
		if x, ok := asInt64(v); ok && x > n {
			return &FieldViolation{Key: "max", Message: "must be at most " + strconv.FormatInt(n, 10)}
		}
		return nil
	}}
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
	r, ok := v.(interface{ Rules() Rules })
	if !ok {
		return nil
	}
	for _, rule := range r.Rules() {
		if err := rule.check(nil); err != nil {
			var fv *FieldViolation
			if ok := asFieldViolation(err, &fv); ok {
				return fv
			}
			return &FieldViolation{Key: rule.key, Message: err.Error()}
		}
	}
	return nil
}

// asFieldViolation unwraps a field violation.
func asFieldViolation(err error, out **FieldViolation) bool {
	if fv, ok := err.(*FieldViolation); ok {
		*out = fv
		return true
	}
	return false
}

// asInt64 converts the integer and float kinds to int64.
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
	case float32:
		return int64(x), true
	case float64:
		return int64(x), true
	}
	return 0, false
}
