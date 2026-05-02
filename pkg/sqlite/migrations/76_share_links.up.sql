CREATE TABLE `share_links` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `token_hash` BLOB NOT NULL UNIQUE,
    `share_type` VARCHAR(16) NOT NULL,
    `scene_id` INTEGER,
    `image_id` INTEGER,
    `performer_id` INTEGER,
    `expires_at` DATETIME,
    `view_limit` INTEGER,
    `view_count` INTEGER NOT NULL DEFAULT 0,
    `revoked` BOOLEAN NOT NULL DEFAULT 0,
    `note` TEXT,
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL,
    FOREIGN KEY(`scene_id`) REFERENCES `scenes`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`image_id`) REFERENCES `images`(`id`) ON DELETE CASCADE,
    FOREIGN KEY(`performer_id`) REFERENCES `performers`(`id`) ON DELETE CASCADE
);

CREATE INDEX `index_share_links_token_hash` ON `share_links`(`token_hash`);
CREATE INDEX `index_share_links_scene_id` ON `share_links`(`scene_id`);
CREATE INDEX `index_share_links_image_id` ON `share_links`(`image_id`);
CREATE INDEX `index_share_links_performer_id` ON `share_links`(`performer_id`);
