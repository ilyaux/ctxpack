package com.acme.billing.domain;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigDecimal;

import org.junit.jupiter.api.Test;

public class CommissionServiceTest {
    @Test
    void previewsCommissionForRetailMarket() {
        CommissionRequest request = new CommissionRequest();
        request.setAmount(new BigDecimal("100.00"));
        request.setMarket("retail");

        CommissionResponse response = new CommissionService().previewCommission(request);
        assertEquals(new BigDecimal("2.50"), response.getCommission());
    }
}
