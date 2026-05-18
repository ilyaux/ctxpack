package com.acme.billing.domain;

import java.math.BigDecimal;

public class CommissionResponse {
    private final BigDecimal commission;

    public CommissionResponse(BigDecimal commission) {
        this.commission = commission;
    }

    public BigDecimal getCommission() {
        return commission;
    }
}
