package service

import (
	"context"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"
)

func verdicts(ctx context.Context, cascade provider.Cascade) ([]DoctorProvider, string) {
	var reported []DoctorProvider
	var titular string
	for _, attempt := range cascade.Diagnose(ctx) {
		reported = append(reported, DoctorProvider{
			Name:   attempt.Name,
			Ready:  attempt.Ready,
			Model:  attempt.ModelID,
			Reason: attempt.Reason,
			Action: attempt.Action,
		})
		if attempt.Ready && titular == "" {
			titular = attempt.Name
		}
	}
	return reported, titular
}
