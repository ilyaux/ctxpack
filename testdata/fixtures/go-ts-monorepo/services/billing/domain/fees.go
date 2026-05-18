package domain

func CalculateCommission(amountCents int64, market string) int64 {
	if market == "enterprise" {
		return amountCents / 200
	}
	return amountCents / 100
}
