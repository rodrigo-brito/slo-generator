package slo

import (
	"log"
	"strings"

	methods "github.com/globocom/slo-generator/methods"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

type SLOSpec struct {
	SLOS []SLO
}

type ExprBlock struct {
	AlertMethod string `yaml:"alertMethod"`
	Expr        string `yaml:"expr"`
}

func (block *ExprBlock) ComputeExpr(window, le string) string {
	replacer := strings.NewReplacer("$window", window, "$le", le)
	return replacer.Replace(block.Expr)
}

type SLO struct {
	Name       string `yaml:"name"`
	Objectives Objectives

	ErrorRateRecord ExprBlock         `yaml:"errorRateRecord"`
	LatencyRecord   ExprBlock         `yaml:"latencyRecord"`
	Annotations     map[string]string `yaml:"annotations"`
}

type Objectives struct {
	Availability float64                 `yaml:"availability"`
	Latency      []methods.LatencyTarget `yaml:"latency"`
}

func (slo SLO) GenerateAlertRules() []rulefmt.Rule {
	alertRules := []rulefmt.Rule{}

	errorMethod := methods.Get(slo.ErrorRateRecord.AlertMethod)
	if errorMethod != nil {
		errorRules := errorMethod.AlertForError(slo.Name, slo.Objectives.Availability, slo.Annotations)
		alertRules = append(alertRules, errorRules...)
	}

	latencyMethod := methods.Get(slo.LatencyRecord.AlertMethod)
	if latencyMethod != nil {
		latencyRules := latencyMethod.AlertForLatency(slo.Name, slo.Objectives.Latency, slo.Annotations)
		alertRules = append(alertRules, latencyRules...)
	}

	return alertRules
}

func (slo SLO) GenerateGroupRules() []rulefmt.RuleGroup {
	rules := []rulefmt.RuleGroup{}

	for _, sample := range defaultSamples {
		interval, err := model.ParseDuration(sample.Interval)
		if err != nil {
			log.Fatal(err)
		}
		ruleGroup := rulefmt.RuleGroup{
			Name:     "slo:" + slo.Name + ":" + sample.Name,
			Interval: interval,
			Rules:    []rulefmt.Rule{},
		}

		for _, bucket := range sample.Buckets {
			errorRateRecord := rulefmt.Rule{
				Record: "slo:service_errors_total:ratio_rate_" + bucket,
				Expr:   slo.ErrorRateRecord.ComputeExpr(bucket, ""),
				Labels: map[string]string{
					"service": slo.Name,
				},
			}

			ruleGroup.Rules = append(ruleGroup.Rules, errorRateRecord)

			for _, latencyBucket := range slo.Objectives.Latency {
				latencyRateRecord := rulefmt.Rule{
					Record: "slo:service_latency:ratio_rate_" + bucket,
					Expr:   slo.LatencyRecord.ComputeExpr(bucket, latencyBucket.LE),
					Labels: map[string]string{
						"service": slo.Name,
						"le":      latencyBucket.LE,
					},
				}

				ruleGroup.Rules = append(ruleGroup.Rules, latencyRateRecord)
			}

		}

		rules = append(rules, ruleGroup)
	}

	return rules
}