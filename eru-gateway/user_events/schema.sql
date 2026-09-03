create table if not exists erugateway_user_events (
    id           bigserial primary key,
    request_id   text,
    trace_id     text,
    user_id      text,
    host         text,
    path         text,
    method       text,
    status       int,
    duration_ms  int,
    target_host  text,
    client_ip    text,
    headers      jsonb,
    request_time timestamptz not null default now()
);

create index if not exists erugateway_user_events_request_time_brin
    on erugateway_user_events using brin (request_time);

create index if not exists erugateway_user_events_user_id_time
    on erugateway_user_events (user_id, request_time desc);
