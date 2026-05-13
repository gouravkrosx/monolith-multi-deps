-- Task Manager schema
CREATE DATABASE IF NOT EXISTS taskmanager
  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE taskmanager;

CREATE TABLE IF NOT EXISTS roles (
  id          INT AUTO_INCREMENT PRIMARY KEY,
  name        VARCHAR(32) NOT NULL UNIQUE,
  description VARCHAR(255) DEFAULT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS users (
  id            CHAR(36) PRIMARY KEY,
  username      VARCHAR(64) NOT NULL UNIQUE,
  email         VARCHAR(128) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role_id       INT NOT NULL,
  external_id   INT DEFAULT NULL, -- maps to JSONPlaceholder user id
  is_active     TINYINT(1) NOT NULL DEFAULT 1,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS tasks (
  id          CHAR(36) PRIMARY KEY,
  title       VARCHAR(255) NOT NULL,
  description TEXT,
  status      ENUM('pending','in_progress','completed','cancelled') NOT NULL DEFAULT 'pending',
  priority    ENUM('low','medium','high','urgent') NOT NULL DEFAULT 'medium',
  due_date    DATETIME DEFAULT NULL,
  owner_id    CHAR(36) NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_tasks_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE,
  INDEX idx_tasks_status (status),
  INDEX idx_tasks_owner (owner_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS task_assignments (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  task_id     CHAR(36) NOT NULL,
  user_id     CHAR(36) NOT NULL,
  assigned_by CHAR(36) NOT NULL,
  assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_task_user (task_id, user_id),
  CONSTRAINT fk_assign_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_assign_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_assign_by   FOREIGN KEY (assigned_by) REFERENCES users(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS task_history (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  task_id    CHAR(36) NOT NULL,
  changed_by CHAR(36) NOT NULL,
  field      VARCHAR(64) NOT NULL,
  old_value  TEXT,
  new_value  TEXT,
  changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_hist_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_hist_user FOREIGN KEY (changed_by) REFERENCES users(id),
  INDEX idx_hist_task (task_id)
) ENGINE=InnoDB;

-- Seed roles
INSERT IGNORE INTO roles (id, name, description) VALUES
  (1, 'admin', 'Full access — manage users and all tasks'),
  (2, 'user',  'Standard user — manage own tasks');
