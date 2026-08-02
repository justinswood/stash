-- Per-user API keys. Replaces the single global config key as the way an
-- automated client authenticates: a key now belongs to an account and carries
-- that account's role, so disabling or demoting the owner takes its keys with it.
--
-- Only the SHA-256 hash of the key is stored; the plaintext is shown once at
-- creation and is irrecoverable afterwards (same scheme as share_links).
-- `prefix` holds the first few plaintext characters purely so the UI can tell
-- keys apart in a list.

CREATE TABLE `user_api_keys` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `user_id` INTEGER NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `key_hash` BLOB NOT NULL UNIQUE,
    `prefix` VARCHAR(12) NOT NULL,
    `last_used_at` DATETIME,
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL,
    FOREIGN KEY(`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
);

CREATE INDEX `index_user_api_keys_key_hash` ON `user_api_keys`(`key_hash`);
CREATE INDEX `index_user_api_keys_user_id` ON `user_api_keys`(`user_id`);
