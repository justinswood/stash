-- Per-user favourites for performers, studios and tags.
--
-- Favourites were a single boolean column on each object, so every account saw
-- and edited the same flag. This is the same problem scene view/o history had
-- before migration 78, and it is solved the same way: a join table keyed on
-- user, backfilled by attributing the existing global state to the first admin.
--
-- The legacy `favorite` columns are deliberately LEFT IN PLACE and are no
-- longer read or written by the application. Dropping them would rewrite three
-- large tables and break any external tooling reading the database directly;
-- they are harmless as a historical record of what was favourited before the
-- split. The join tables are authoritative from here on.

CREATE TABLE `performers_favorites` (
    `user_id` INTEGER NOT NULL,
    `performer_id` INTEGER NOT NULL,
    PRIMARY KEY (`user_id`, `performer_id`),
    FOREIGN KEY(`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`performer_id`) REFERENCES `performers`(`id`) ON DELETE CASCADE
);

CREATE TABLE `studios_favorites` (
    `user_id` INTEGER NOT NULL,
    `studio_id` INTEGER NOT NULL,
    PRIMARY KEY (`user_id`, `studio_id`),
    FOREIGN KEY(`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`studio_id`) REFERENCES `studios`(`id`) ON DELETE CASCADE
);

CREATE TABLE `tags_favorites` (
    `user_id` INTEGER NOT NULL,
    `tag_id` INTEGER NOT NULL,
    PRIMARY KEY (`user_id`, `tag_id`),
    FOREIGN KEY(`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`tag_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE
);

CREATE INDEX `index_performers_favorites_user` ON `performers_favorites`(`user_id`);
CREATE INDEX `index_studios_favorites_user` ON `studios_favorites`(`user_id`);
CREATE INDEX `index_tags_favorites_user` ON `tags_favorites`(`user_id`);

-- Attribute existing favourites to the first admin account, matching what
-- migration 78 did with existing view history. Every other account starts with
-- none. The subselect yields NULL when there is no admin (a system with no
-- credentials configured), and the NOT NULL guard makes the insert a no-op
-- rather than failing.
INSERT INTO `performers_favorites` (`user_id`, `performer_id`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`
FROM `performers`
WHERE `favorite` = 1
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `studios_favorites` (`user_id`, `studio_id`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`
FROM `studios`
WHERE `favorite` = 1
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `tags_favorites` (`user_id`, `tag_id`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`
FROM `tags`
WHERE `favorite` = 1
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;
