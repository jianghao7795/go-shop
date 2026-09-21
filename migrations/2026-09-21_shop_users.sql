-- =============================================================================
-- shop_users 表迁移
-- 目标库: shop (MySQL 8.x, utf8mb4)
-- 说明:
--   商城注册用户表。原表只有 id/username/password/created_at/updated_at/deleted_at，
--   本次新增 mobile/email/nickname/avatar/role/status/last_login_at/last_login_ip，
--   并收紧 username/password 为 NOT NULL、status 改为 tinyint NOT NULL DEFAULT 1。
--   该表与同库另一系统的 users 表（以 mobile 标识）互相独立，故用 shop_ 前缀。
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 一、全新环境：直接建表（幂等，表已存在则跳过）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `shop_users` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `username`      VARCHAR(64)     NOT NULL                COMMENT '登录用户名，唯一',
  `password`      VARCHAR(255)    NOT NULL                COMMENT 'bcrypt 哈希',
  `mobile`        VARCHAR(20)     DEFAULT NULL            COMMENT '手机号',
  `email`         VARCHAR(128)    DEFAULT NULL            COMMENT '邮箱',
  `nickname`      VARCHAR(64)     DEFAULT NULL            COMMENT '昵称，默认取 username',
  `avatar`        VARCHAR(255)    DEFAULT NULL            COMMENT '头像 URL',
  `role`          VARCHAR(20)     DEFAULT 'customer'      COMMENT '角色：customer/admin',
  `status`        TINYINT         NOT NULL DEFAULT 1      COMMENT '状态：1 正常，0 禁用',
  `last_login_at` DATETIME(3)     DEFAULT NULL            COMMENT '最后登录时间',
  `last_login_ip` VARCHAR(45)     DEFAULT NULL            COMMENT '最后登录 IP',
  `created_at`    DATETIME(3)     DEFAULT NULL,
  `updated_at`    DATETIME(3)     DEFAULT NULL,
  `deleted_at`    DATETIME(3)     DEFAULT NULL            COMMENT '软删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_mobile` (`mobile`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商城注册用户';

-- -----------------------------------------------------------------------------
-- 二、旧库升级：表已存在时执行（MySQL 8 不支持 ADD COLUMN IF NOT EXISTS，
--    这些语句对应 GORM AutoMigrate 已完成的变更，供手工迁移/审计参考）
-- -----------------------------------------------------------------------------

-- 2.1 新增字段
ALTER TABLE `shop_users`
  ADD COLUMN `mobile`        VARCHAR(20)  DEFAULT NULL COMMENT '手机号' AFTER `password`,
  ADD COLUMN `email`         VARCHAR(128) DEFAULT NULL COMMENT '邮箱'   AFTER `mobile`,
  ADD COLUMN `nickname`      VARCHAR(64)  DEFAULT NULL COMMENT '昵称'   AFTER `email`,
  ADD COLUMN `avatar`        VARCHAR(255) DEFAULT NULL COMMENT '头像 URL' AFTER `nickname`,
  ADD COLUMN `role`          VARCHAR(20)  DEFAULT 'customer' COMMENT '角色' AFTER `avatar`,
  ADD COLUMN `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '状态' AFTER `role`,
  ADD COLUMN `last_login_at` DATETIME(3)  DEFAULT NULL COMMENT '最后登录时间' AFTER `status`,
  ADD COLUMN `last_login_ip` VARCHAR(45)  DEFAULT NULL COMMENT '最后登录 IP' AFTER `last_login_at`;

-- 2.2 收紧约束
ALTER TABLE `shop_users` MODIFY `username` VARCHAR(64)  NOT NULL;
ALTER TABLE `shop_users` MODIFY `password` VARCHAR(255) NOT NULL;
ALTER TABLE `shop_users` MODIFY `status`   TINYINT      NOT NULL DEFAULT 1;

-- 2.3 索引
CREATE INDEX `idx_mobile` ON `shop_users` (`mobile`);
