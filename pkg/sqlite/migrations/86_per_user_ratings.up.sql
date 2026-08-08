-- Per-user ratings for scenes, images, galleries, performers, studios and
-- groups.
--
-- Ratings were a single column on each object, so every account saw and edited
-- the same stars. This is the last of the shared-state splits — view history
-- went per-user in migration 78, favourites in 83, saved filters in 84 and the
-- interface configuration in 85 — and it is the one behind the original
-- report: a saved filter named "Favs" is `rating100 = 100`, so a second
-- account opening it was shown the admin's five-star scenes.
--
-- Unlike favourites, a rating is a value rather than a flag, so it has to work
-- in ORDER BY as well as WHERE. Both go through a correlated subquery rather
-- than a join, so that unrated objects still appear in a rating sort instead
-- of being dropped from the result entirely.
--
-- The legacy `rating` columns are deliberately LEFT IN PLACE, as the `favorite`
-- columns were in migration 83. Dropping them would rewrite six large tables
-- and break external tooling that reads the database directly. They record the
-- pre-split state and are no longer authoritative — expect them to drift, and
-- do not trust them when querying by hand.

CREATE TABLE `scenes_ratings` (
    `user_id` integer not null,
    `scene_id` integer not null,
    `rating` integer not null,
    primary key (`user_id`, `scene_id`),
    foreign key(`user_id`) references `users`(`id`) on delete cascade,
    foreign key(`scene_id`) references `scenes`(`id`) on delete cascade
);

CREATE TABLE `images_ratings` (
    `user_id` integer not null,
    `image_id` integer not null,
    `rating` integer not null,
    primary key (`user_id`, `image_id`),
    foreign key(`user_id`) references `users`(`id`) on delete cascade,
    foreign key(`image_id`) references `images`(`id`) on delete cascade
);

CREATE TABLE `galleries_ratings` (
    `user_id` integer not null,
    `gallery_id` integer not null,
    `rating` integer not null,
    primary key (`user_id`, `gallery_id`),
    foreign key(`user_id`) references `users`(`id`) on delete cascade,
    foreign key(`gallery_id`) references `galleries`(`id`) on delete cascade
);

CREATE TABLE `performers_ratings` (
    `user_id` integer not null,
    `performer_id` integer not null,
    `rating` integer not null,
    primary key (`user_id`, `performer_id`),
    foreign key(`user_id`) references `users`(`id`) on delete cascade,
    foreign key(`performer_id`) references `performers`(`id`) on delete cascade
);

CREATE TABLE `studios_ratings` (
    `user_id` integer not null,
    `studio_id` integer not null,
    `rating` integer not null,
    primary key (`user_id`, `studio_id`),
    foreign key(`user_id`) references `users`(`id`) on delete cascade,
    foreign key(`studio_id`) references `studios`(`id`) on delete cascade
);

CREATE TABLE `groups_ratings` (
    `user_id` integer not null,
    `group_id` integer not null,
    `rating` integer not null,
    primary key (`user_id`, `group_id`),
    foreign key(`user_id`) references `users`(`id`) on delete cascade,
    foreign key(`group_id`) references `groups`(`id`) on delete cascade
);

-- The primary key (user_id, <fk>) already serves the correlated subquery used
-- by rating filters and sorts, which looks up one object for one user. These
-- cover the other direction — "everything this user rated" — used when loading
-- a page of results.
CREATE INDEX `index_scenes_ratings_user` ON `scenes_ratings`(`user_id`);
CREATE INDEX `index_images_ratings_user` ON `images_ratings`(`user_id`);
CREATE INDEX `index_galleries_ratings_user` ON `galleries_ratings`(`user_id`);
CREATE INDEX `index_performers_ratings_user` ON `performers_ratings`(`user_id`);
CREATE INDEX `index_studios_ratings_user` ON `studios_ratings`(`user_id`);
CREATE INDEX `index_groups_ratings_user` ON `groups_ratings`(`user_id`);

-- Attribute existing ratings to the first admin, matching what migrations 78,
-- 83 and 84 did. Every other account starts unrated. The subselect yields NULL
-- when there is no admin (an instance with authentication disabled) and the
-- NOT NULL guard makes each insert a no-op rather than failing.
INSERT INTO `scenes_ratings` (`user_id`, `scene_id`, `rating`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`, `rating`
FROM `scenes`
WHERE `rating` IS NOT NULL
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `images_ratings` (`user_id`, `image_id`, `rating`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`, `rating`
FROM `images`
WHERE `rating` IS NOT NULL
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `galleries_ratings` (`user_id`, `gallery_id`, `rating`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`, `rating`
FROM `galleries`
WHERE `rating` IS NOT NULL
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `performers_ratings` (`user_id`, `performer_id`, `rating`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`, `rating`
FROM `performers`
WHERE `rating` IS NOT NULL
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `studios_ratings` (`user_id`, `studio_id`, `rating`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`, `rating`
FROM `studios`
WHERE `rating` IS NOT NULL
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;

INSERT INTO `groups_ratings` (`user_id`, `group_id`, `rating`)
SELECT (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1), `id`, `rating`
FROM `groups`
WHERE `rating` IS NOT NULL
  AND (SELECT `id` FROM `users` WHERE `role` = 'ADMIN' ORDER BY `id` LIMIT 1) IS NOT NULL;
