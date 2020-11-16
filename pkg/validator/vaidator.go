package validator

import (
	"fmt"
	"github.com/influxdata/promql/v2"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"gopkg.in/yaml.v3"
	"strings"
	"time"
)

func newExpressionDoesNotUseOlderDataThan(paramsConfig yaml.Node) (Validator, error) {
	params := struct {
		Limit time.Duration `yaml:"limit"`
	}{}
	if err := paramsConfig.Decode(&params); err != nil {
		return nil, err
	}
	if params.Limit == time.Duration(0) {
		return nil, fmt.Errorf("missing limit")
	}
	return &expressionDoesNotUseOlderDataThan{limit: params.Limit}, nil
}

type expressionDoesNotUseOlderDataThan struct {
	limit time.Duration
}

func (h expressionDoesNotUseOlderDataThan) String() string {
	return fmt.Sprintf("Expression does not use data older than `%s`", h.limit)
}

func (h expressionDoesNotUseOlderDataThan) Validate(rule rulefmt.Rule) []error {
	expr, err := promql.ParseExpr(rule.Expr)
	if err != nil {
		return []error{fmt.Errorf("failed to parse expression `%s`: %s", rule.Expr, err)}
	}
	var errs []error
	promql.Inspect(expr, func(n promql.Node, ns []promql.Node) error {
		// TODO(FUSAKLA) Having range query in subquery should have the time added.
		switch v := n.(type) {
		case *promql.MatrixSelector:
			if v.Range+v.Offset > h.limit {
				errs = append(errs, fmt.Errorf("expr uses `%s` old data which is more than limit `%s`", v.Range+v.Offset, h.limit))
			}
		case *promql.VectorSelector:
			if v.Offset > h.limit {
				errs = append(errs, fmt.Errorf("expr uses `%s` old data which is more than limit `%s`", v.Offset, h.limit))
			}
		case *promql.SubqueryExpr:
			if v.Range+v.Offset > h.limit {
				errs = append(errs, fmt.Errorf("expr uses `%s` old data which is more than limit `%s`", v.Range+v.Offset, h.limit))
			}
		}
		return nil
	})
	return errs
}

func newExpressionDoesNotUseLabels(paramsConfig yaml.Node) (Validator, error) {
	params := struct {
		Labels []string `yaml:"labels"`
	}{}
	if err := paramsConfig.Decode(&params); err != nil {
		return nil, err
	}
	if len(params.Labels) == 0 {
		return nil, fmt.Errorf("missing labels")
	}
	return &expressionDoesNotUseLabels{labels: params.Labels}, nil
}

type expressionDoesNotUseLabels struct {
	labels []string
}

func (h expressionDoesNotUseLabels) String() string {
	return fmt.Sprintf("None of the `%s` labels is used in the expression", strings.Join(h.labels, "`,`"))
}

func (h expressionDoesNotUseLabels) Validate(rule rulefmt.Rule) []error {
	expr, err := promql.ParseExpr(rule.Expr)
	if err != nil {
		return []error{fmt.Errorf("failed to parse expression `%s`: %s", rule.Expr, err)}
	}
	var usedLabels []string
	var errs []error
	promql.Inspect(expr, func(n promql.Node, ns []promql.Node) error {
		switch v := n.(type) {
		case *promql.MatrixSelector:
			for _, m := range v.LabelMatchers {
				usedLabels = append(usedLabels, m.Name)
			}
		case *promql.AggregateExpr:
			for _, m := range v.Grouping {
				usedLabels = append(usedLabels, m)
			}
		case *promql.VectorSelector:
			for _, m := range v.LabelMatchers {
				usedLabels = append(usedLabels, m.Name)
			}
		case *promql.BinaryExpr:
			if v.VectorMatching != nil {
				for _, m := range v.VectorMatching.Include {
					usedLabels = append(usedLabels, m)
				}
				for _, m := range v.VectorMatching.MatchingLabels {
					usedLabels = append(usedLabels, m)
				}
			}
		}
		return nil
	})
	for _, l := range usedLabels {
		for _, n := range h.labels {
			if l == n {
				errs = append(errs, fmt.Errorf("forbidden label `%s` used in expression", l))
			}
		}
	}
	return errs
}