-- Per-user interface configuration.
--
-- The UI configuration — front page layout, default filters, theme settings,
-- interface preferences — lived in a single blob in the instance config file,
-- so every account rendered the admin's home page. Combined with globally
-- shared saved filters (migration 84), a second account opening the site was
-- shown the admin's front page pointing at the admin's "Favs" filter.
--
-- The instance config file keeps its `ui` blob and is NOT removed: it is still
-- the source for the values the server itself reads at startup, notably
-- minimumPlayPercent, which is handed to the DLNA service in manager.Initialize
-- long before any request exists to carry a user. Only the per-request view of
-- the UI configuration becomes per-user.
--
-- The existing blob is copied to the first admin by the post-migration in
-- 85_postmigrate.go, which is where it has to happen — a config file is not
-- reachable from SQL. Every other account starts empty and therefore gets the
-- stock Stash defaults rather than inheriting somebody else's layout.

CREATE TABLE `user_ui_config` (
    `user_id` integer not null primary key,
    `config` blob not null,
    foreign key(`user_id`) references `users`(`id`) on delete cascade
);
