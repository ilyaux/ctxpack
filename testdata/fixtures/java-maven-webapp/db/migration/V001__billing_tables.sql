CREATE TABLE billing_invoice (
  id BIGINT PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL,
  total_amount DECIMAL(18, 2) NOT NULL
);
