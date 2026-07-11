-- User billing always uses the requested model. Keep the column for rolling-deploy API compatibility.
UPDATE channels
SET billing_model_source = 'requested'
WHERE billing_model_source IS DISTINCT FROM 'requested';

ALTER TABLE channels
    ALTER COLUMN billing_model_source SET DEFAULT 'requested';
