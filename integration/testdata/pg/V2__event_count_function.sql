CREATE FUNCTION event_count() RETURNS bigint AS $$
BEGIN
    RETURN (SELECT count(*) FROM events);
END;
$$ LANGUAGE plpgsql;

INSERT INTO events (id, kind) VALUES (1, 'created');
