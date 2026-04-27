ALTER TABLE datasources ADD COLUMN version VARCHAR(128) DEFAULT '' AFTER is_enabled;
