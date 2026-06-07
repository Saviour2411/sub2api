-- Add channel-level default token pricing for models that miss channel/LiteLLM pricing.
ALTER TABLE channels ADD COLUMN IF NOT EXISTS default_pricing_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE channels ADD COLUMN IF NOT EXISTS default_input_price NUMERIC(20,12);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS default_output_price NUMERIC(20,12);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS default_cache_write_price NUMERIC(20,12);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS default_cache_read_price NUMERIC(20,12);

COMMENT ON COLUMN channels.default_pricing_enabled IS 'Enable channel default token pricing when model-specific and global pricing miss';
COMMENT ON COLUMN channels.default_input_price IS 'Default input token price (USD per token) for unmatched models';
COMMENT ON COLUMN channels.default_output_price IS 'Default output token price (USD per token) for unmatched models';
COMMENT ON COLUMN channels.default_cache_write_price IS 'Default cache write token price (USD per token) for unmatched models';
COMMENT ON COLUMN channels.default_cache_read_price IS 'Default cache read token price (USD per token) for unmatched models';
