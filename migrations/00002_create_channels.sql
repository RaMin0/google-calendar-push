create table channels (
  id character varying primary key,
  user_id character varying references users(id) on delete cascade,
  token character varying not null,
  last_sync_token character varying not null,
  resource_id character varying not null
);

---- create above / drop below ----

drop table channels;
