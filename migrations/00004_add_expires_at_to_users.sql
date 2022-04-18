alter table users add column expires_at timestamp without time zone not null default (current_timestamp at time zone 'utc');

---- create above / drop below ----

alter table users drop column expires_at;
