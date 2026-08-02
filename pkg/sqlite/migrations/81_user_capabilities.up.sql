-- Per-user capability overrides.
--
-- Roles remain, but as presets over a capability set (see
-- pkg/models/capability.go). This table records only *departures* from a user's
-- role preset: effective = preset(role) + granted - revoked.
--
-- The table starts empty, so every existing account keeps exactly the
-- permissions its role gave it before capabilities existed. Nothing changes
-- until an override is added.
--
-- `granted` distinguishes the two directions: 1 adds a capability the role
-- preset lacks, 0 removes one the preset includes.

CREATE TABLE `user_capabilities` (
    `user_id` INTEGER NOT NULL,
    `capability` VARCHAR(64) NOT NULL,
    `granted` BOOLEAN NOT NULL,
    PRIMARY KEY (`user_id`, `capability`),
    FOREIGN KEY(`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
);

CREATE INDEX `index_user_capabilities_user_id` ON `user_capabilities`(`user_id`);
