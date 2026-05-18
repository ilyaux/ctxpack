package com.acme.auth;

import org.springframework.web.bind.annotation.RestController;

@RestController
public class AuthController {
    public boolean validateSession(String token) {
        return token != null && token.length() > 20;
    }
}
