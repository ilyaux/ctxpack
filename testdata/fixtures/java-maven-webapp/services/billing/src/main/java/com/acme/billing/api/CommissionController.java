package com.acme.billing.api;

import com.acme.billing.domain.CommissionRequest;
import com.acme.billing.domain.CommissionResponse;
import com.acme.billing.domain.CommissionService;

import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/billing")
public class CommissionController {
    private final CommissionService commissionService;

    public CommissionController(CommissionService commissionService) {
        this.commissionService = commissionService;
    }

    @PostMapping("/commission/preview")
    public CommissionResponse previewCommission(@RequestBody CommissionRequest request) {
        return commissionService.previewCommission(request);
    }
}
