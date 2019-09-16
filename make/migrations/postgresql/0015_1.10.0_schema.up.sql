/** Add table for immutable tag  **/
CREATE TABLE immutable_tag_rule
(
  id            SERIAL PRIMARY KEY NOT NULL,
  project_id    int NOT NULL,
  repo_filter  text,
  tag_filter   text,
  enabled       boolean default true NOT NULL,
  creation_time timestamp default CURRENT_TIMESTAMP,
  update_time   timestamp default CURRENT_TIMESTAMP
)