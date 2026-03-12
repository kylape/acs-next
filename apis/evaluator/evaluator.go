package evaluator

import (
	"fmt"

	commonv1 "acs-next.stackrox.io/apis/common/v1"
)

// Result represents the outcome of evaluating a policy against a context.
type Result struct {
	Matched  bool
	Messages []string
}

// EvaluateSections evaluates PolicySections against the given context.
// Sections are ANDed together: all sections must match for the policy to fire.
// Within a section, groups are ANDed: all groups must match for the section to fire.
func EvaluateSections(sections []commonv1.PolicySection, ctx *EvalContext) Result {
	var allMessages []string

	for _, section := range sections {
		sectionResult := evaluateSection(section, ctx)
		if !sectionResult.Matched {
			return Result{Matched: false}
		}
		allMessages = append(allMessages, sectionResult.Messages...)
	}

	return Result{Matched: true, Messages: allMessages}
}

func evaluateSection(section commonv1.PolicySection, ctx *EvalContext) Result {
	var messages []string

	for _, group := range section.PolicyGroups {
		groupResult := evaluateGroup(group, ctx)
		// Groups within a section are ANDed
		if !groupResult.Matched {
			return Result{Matched: false}
		}
		messages = append(messages, groupResult.Messages...)
	}

	return Result{Matched: true, Messages: messages}
}

func evaluateGroup(group commonv1.PolicyGroup, ctx *EvalContext) Result {
	handler := GetFieldHandler(group.FieldName)
	if handler == nil {
		return Result{Matched: false, Messages: []string{fmt.Sprintf("unsupported field: %s", group.FieldName)}}
	}

	var values []string
	for _, v := range group.Values {
		values = append(values, v.Value)
	}

	var matched bool
	if len(values) == 0 {
		// No values = boolean check (e.g., "Privileged Container" with no values = check if privileged)
		matched = handler(ctx, nil)
	} else if group.BooleanOperator == commonv1.BooleanOperatorAnd {
		// AND: all values must match
		matched = true
		for _, v := range values {
			if !handler(ctx, []string{v}) {
				matched = false
				break
			}
		}
	} else {
		// OR (default): any value must match
		matched = handler(ctx, values)
	}

	// Apply negation
	if group.Negate {
		matched = !matched
	}

	if matched {
		msg := fmt.Sprintf("field %q matched", group.FieldName)
		if len(values) > 0 {
			msg = fmt.Sprintf("field %q matched values %v", group.FieldName, values)
		}
		return Result{Matched: true, Messages: []string{msg}}
	}

	return Result{Matched: false}
}
