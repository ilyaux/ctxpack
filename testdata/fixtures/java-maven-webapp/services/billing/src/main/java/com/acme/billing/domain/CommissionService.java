package com.acme.billing.domain;

import java.math.BigDecimal;
import java.math.RoundingMode;

import org.springframework.stereotype.Service;

@Service
public class CommissionService {
    public CommissionResponse previewCommission(CommissionRequest request) {
        BigDecimal rate = rateForMarket(request.getMarket());
        BigDecimal commission = request.getAmount().multiply(rate).setScale(2, RoundingMode.HALF_UP);
        return new CommissionResponse(commission);
    }

    private BigDecimal rateForMarket(String market) {
        if ("enterprise".equalsIgnoreCase(market)) {
            return new BigDecimal("0.015");
        }
        return new BigDecimal("0.025");
    }
}
