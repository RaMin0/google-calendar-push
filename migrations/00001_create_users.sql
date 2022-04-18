create table users (
  id character varying primary key,
  access_token character varying not null,
  refresh_token character varying not null
);

---- create above / drop below ----

drop table users;
