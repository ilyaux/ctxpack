package com.acme.billing.domain;

import java.math.BigDecimal;

public class CommissionRequest {
    private BigDecimal amount;
    private String market;

    public BigDecimal getAmount() {
        return amount;
    }

    public void setAmount(BigDecimal amount) {
        this.amount = amount;
    }

    public String getMarket() {
        return market;
    }

    public void setMarket(String market) {
        this.market = market;
    }
}
