export type CommissionRequest = {
  amountCents: number;
  market: string;
};

export type CommissionResponse = {
  commissionCents: number;
};

export async function previewCommission(
  request: CommissionRequest,
): Promise<CommissionResponse> {
  const response = await fetch("/billing/commission/preview", {
    method: "POST",
    body: JSON.stringify(request),
  });
  return response.json();
}
