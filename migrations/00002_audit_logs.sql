-- +goose Up
-- No foreign keys: the log must outlive the records it describes, including purged ones.
CREATE TABLE audit_logs (
    id         varchar(100) PRIMARY KEY,
    entity     varchar(20)  NOT NULL,
    entity_id  varchar(100) NOT NULL,
    action     varchar(20)  NOT NULL,
    actor_id   varchar(100),
    actor_name varchar(150),
    changes    text,
    ip         varchar(45),
    user_agent varchar(255),
    request_id varchar(100),
    tenant_id  varchar(100),
    created_at timestamptz  NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_entity_idx ON audit_logs (entity);
CREATE INDEX audit_logs_entity_id_idx ON audit_logs (entity_id);
CREATE INDEX audit_logs_tenant_id_idx ON audit_logs (tenant_id);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at);

-- Append-only (ISO/IEC 27001 A.8.15). A trigger rather than role grants so it holds whatever
-- role the service connects as; the service should still not own the table, since an owner
-- can drop the trigger.
-- +goose StatementBegin
CREATE FUNCTION audit_logs_reject_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_logs_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_reject_change();

CREATE TRIGGER audit_logs_no_truncate
    BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_reject_change();

-- +goose Down
DROP TRIGGER audit_logs_no_truncate ON audit_logs;
DROP TRIGGER audit_logs_no_update_delete ON audit_logs;
DROP FUNCTION audit_logs_reject_change();
DROP TABLE audit_logs;
