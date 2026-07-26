-- Per-user scene view/o history.
-- Existing global history is attributed to the first admin account.
-- (Per-user resume position / play_duration is a later phase; those columns
--  remain on the scenes table and shared for now.)

PRAGMA foreign_keys=OFF;

-- add a nullable user_id to the scene history tables (nullable so ALTER ADD
-- COLUMN is permitted; backfilled below)
ALTER TABLE `scenes_view_dates` ADD COLUMN `user_id` INTEGER REFERENCES `users`(`id`) ON DELETE CASCADE;
ALTER TABLE `scenes_o_dates` ADD COLUMN `user_id` INTEGER REFERENCES `users`(`id`) ON DELETE CASCADE;

UPDATE `scenes_view_dates`
  SET `user_id` = (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1)
  WHERE `user_id` IS NULL;
UPDATE `scenes_o_dates`
  SET `user_id` = (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1)
  WHERE `user_id` IS NULL;

CREATE INDEX `index_scenes_view_dates_user` ON `scenes_view_dates`(`user_id`);
CREATE INDEX `index_scenes_o_dates_user` ON `scenes_o_dates`(`user_id`);

PRAGMA foreign_keys=ON;
