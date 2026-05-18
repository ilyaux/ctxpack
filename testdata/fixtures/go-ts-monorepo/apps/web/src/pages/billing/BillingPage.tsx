import { previewCommission } from "@ctxpack-fixture/api-client/billing";

export function BillingPage() {
  async function onPreview() {
    return previewCommission({ amountCents: 10000, market: "retail" });
  }

  return <button onClick={onPreview}>Preview commission</button>;
}
