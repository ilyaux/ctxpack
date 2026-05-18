package com.acme.web;

import java.math.BigDecimal;

import javax.inject.Named;

@Named
public class BillingBean {
    private BigDecimal amount;
    private BigDecimal previewCommissionResult;

    public void previewCommission() {
        previewCommissionResult = amount.multiply(new BigDecimal("0.025"));
    }
}
