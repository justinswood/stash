-- User groups carrying content visibility restrictions.
--
-- A group names a set of content that its members must not see. Restrictions
-- are subtractive and additive across groups: a user in several groups is
-- restricted by the union of their exclusions, so adding a group can only ever
-- hide more, never reveal something another group hid.
--
-- Named user_groups, not groups: `groups` is already taken by the scene
-- collection feature (pkg/models/group.go).
--
-- All four tables start empty, so nothing is hidden from anyone until a group
-- is created and a user is assigned to it.

CREATE TABLE `user_groups` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `name` VARCHAR(255) NOT NULL UNIQUE,
    `description` TEXT,
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL
);

CREATE TABLE `user_group_members` (
    `user_id` INTEGER NOT NULL,
    `group_id` INTEGER NOT NULL,
    PRIMARY KEY (`user_id`, `group_id`),
    FOREIGN KEY(`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`group_id`) REFERENCES `user_groups`(`id`) ON DELETE CASCADE
);

-- Performer genders hidden from members. Stored as the enum's string form, the
-- same representation as performers.gender.
CREATE TABLE `user_group_excluded_genders` (
    `group_id` INTEGER NOT NULL,
    `gender` VARCHAR(20) NOT NULL,
    PRIMARY KEY (`group_id`, `gender`),
    FOREIGN KEY(`group_id`) REFERENCES `user_groups`(`id`) ON DELETE CASCADE
);

-- Tags hidden from members. Chosen explicitly rather than matched by name:
-- a substring search for "trans" also catches "Transformation" and
-- "Transparent Clothing", which are unrelated.
CREATE TABLE `user_group_excluded_tags` (
    `group_id` INTEGER NOT NULL,
    `tag_id` INTEGER NOT NULL,
    PRIMARY KEY (`group_id`, `tag_id`),
    FOREIGN KEY(`group_id`) REFERENCES `user_groups`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`tag_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE
);

CREATE INDEX `index_user_group_members_user` ON `user_group_members`(`user_id`);
CREATE INDEX `index_user_group_excluded_tags_group` ON `user_group_excluded_tags`(`group_id`);
