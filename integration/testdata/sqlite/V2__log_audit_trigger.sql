CREATE TABLE log_audit (
    id   INTEGER PRIMARY KEY,
    note TEXT NOT NULL
);

CREATE TRIGGER logs_after_insert AFTER INSERT ON logs
BEGIN
    INSERT INTO log_audit (note) VALUES ('inserted');
    UPDATE log_audit SET note = 'updated' WHERE id = 1;
END;
