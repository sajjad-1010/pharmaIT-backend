CREATE TABLE campaign_join_requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID        NOT NULL REFERENCES discount_campaigns(id),
    pharmacy_id UUID        NOT NULL REFERENCES pharmacies(user_id),
    message     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_campaign_join_requests_campaign_pharmacy UNIQUE (campaign_id, pharmacy_id)
);

CREATE INDEX idx_campaign_join_requests_campaign ON campaign_join_requests (campaign_id, created_at DESC);
CREATE INDEX idx_campaign_join_requests_pharmacy ON campaign_join_requests (pharmacy_id, created_at DESC);
