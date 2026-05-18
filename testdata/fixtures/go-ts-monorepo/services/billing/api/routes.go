package api

import "example.com/acme/services/billing/domain"

type CommissionRequest struct {
	AmountCents int64
	Market      string
}

type CommissionResponse struct {
	CommissionCents int64
}

func RegisterBillingRoutes(router Router) {
	router.Post("/billing/commission/preview", PreviewCommission)
}

func PreviewCommission(req CommissionRequest) CommissionResponse {
	return CommissionResponse{
		CommissionCents: domain.CalculateCommission(req.AmountCents, req.Market),
	}
}

type Router interface {
	Post(path string, handler any)
}
