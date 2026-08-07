-- Per-user saved filters.
--
-- saved_filters had no owner, so every account saw, edited and deleted every
-- other account's filters. This is the same problem view history had before
-- migration 78 and favourites had before 83, and it is solved the same way:
-- an owner column, backfilled by attributing existing rows to the first admin.
--
-- user_id is deliberately NULLABLE. When authentication is disabled,
-- authenticateHandler lets requests through with no user at all
-- (authentication.go), so there is no id to scope by — and there is also no
-- privacy boundary to enforce, because there are no accounts. Those filters
-- are stored with a NULL owner and match only each other. Making the column
-- NOT NULL would either break filter creation on such an instance or require
-- a sentinel id that violates the foreign key.
--
-- SQLite cannot add a column with a foreign key in place, so the table is
-- rebuilt exactly as migration 49 rebuilt it.

PRAGMA foreign_keys=OFF;

CREATE TABLE `saved_filters_new` (
  `id` integer not null primary key autoincrement,
  `user_id` integer,
  `name` varchar(510) not null,
  `mode` varchar(255) not null,
  `find_filter` blob,
  `object_filter` blob,
  `ui_options` blob,
  foreign key(`user_id`) references `users`(`id`) on delete cascade
);

-- Existing filters go to the first admin. Every other account starts with
-- none, which is the point: a filter named "Favs" on one account must not
-- appear on another. The subselect yields NULL where no admin exists — an
-- instance with authentication disabled — which is exactly the unowned state
-- described above, so no COALESCE or sentinel is needed.
INSERT INTO `saved_filters_new`
  (`id`, `user_id`, `name`, `mode`, `find_filter`, `object_filter`, `ui_options`)
  SELECT
    `id`,
    (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1),
    `name`,
    `mode`,
    `find_filter`,
    `object_filter`,
    `ui_options`
  FROM `saved_filters`;

DROP INDEX IF EXISTS `index_saved_filters_on_mode_name_unique`;
DROP TABLE `saved_filters`;
ALTER TABLE `saved_filters_new` RENAME TO `saved_filters`;

-- The uniqueness constraint must include the owner. Without user_id here, two
-- accounts could not both keep a filter called "Favs" for the same mode, which
-- is exactly what this migration exists to allow. Note SQLite treats NULLs as
-- distinct in a UNIQUE index, so the unowned (auth-disabled) case relies on
-- there being a single logical user rather than on this constraint.
CREATE UNIQUE INDEX `index_saved_filters_on_user_mode_name_unique`
  ON `saved_filters` (`user_id`, `mode`, `name`);

CREATE INDEX `index_saved_filters_user` ON `saved_filters` (`user_id`);

PRAGMA foreign_keys=ON;
