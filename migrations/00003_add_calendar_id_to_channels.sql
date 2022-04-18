alter table channels add column calendar_id character varying not null;

---- create above / drop below ----

alter table channels drop column calendar_id;
