CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  username VARCHAR(60) NOT NULL UNIQUE,
  password_hash VARCHAR(220) NOT NULL,
  phone VARCHAR(30) NOT NULL,
  email VARCHAR(120) NOT NULL UNIQUE,
  role ENUM('user', 'admin') NOT NULL DEFAULT 'user',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sessions (
  token CHAR(64) PRIMARY KEY,
  user_id BIGINT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_sessions_user (user_id),
  INDEX idx_sessions_expires (expires_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS captchas (
  id CHAR(32) PRIMARY KEY,
  code VARCHAR(8) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used TINYINT(1) NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_captchas_expires (expires_at)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS posts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  author_id BIGINT NULL,
  title VARCHAR(180) NOT NULL,
  slug VARCHAR(220) NOT NULL UNIQUE,
  summary VARCHAR(500) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  category VARCHAR(80) NOT NULL,
  tags JSON NOT NULL,
  cover_url VARCHAR(500) NOT NULL DEFAULT '',
  status ENUM('draft', 'published') NOT NULL DEFAULT 'draft',
  read_minutes INT NOT NULL DEFAULT 3,
  views BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  published_at TIMESTAMP NULL,
  INDEX idx_posts_author (author_id),
  INDEX idx_posts_status_created (status, created_at),
  INDEX idx_posts_category (category),
  FULLTEXT INDEX ft_posts_search (title, summary, content),
  CONSTRAINT fk_posts_author FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(60) NOT NULL,
  email VARCHAR(120) NOT NULL DEFAULT '',
  content VARCHAR(1000) NOT NULL,
  status ENUM('pending', 'approved') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_comments_status_created (status, created_at)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS site_settings (
  setting_key VARCHAR(80) PRIMARY KEY,
  setting_value TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

INSERT IGNORE INTO site_settings (setting_key, setting_value) VALUES
  ('siteName', 'Tech Blog'), ('siteSubtitle', 'Engineering notes'), ('siteUrl', ''),
  ('seoDescription', '记录软件工程、前端、后端与 DevOps 实践。'),
  ('seoKeywords', '技术博客,Go,Vue,DevOps'), ('defaultOgImage', ''),
  ('footerText', '持续记录，认真构建。');

INSERT IGNORE INTO posts
  (title, slug, summary, content, category, tags, cover_url, status, read_minutes, views, published_at)
VALUES
  (
    'Go 服务的分层实践',
    'go-service-layering',
    '从 handler、service 到 repository，拆出足够清晰但不过度设计的后端边界。',
    'Go 项目可以从最朴素的 handler 开始，然后在业务增长时自然拆出 service 和 repository。关键不是目录数量，而是让依赖方向稳定、数据库细节不泄漏到 HTTP 层。\n\n本文示例采用 database/sql 和 MySQL，保留显式 SQL，便于排查性能问题。对于博客、CMS、运营后台这类系统，清晰的数据模型和可观察的错误处理通常比复杂框架更重要。',
    'Backend',
    JSON_ARRAY('Go', 'API', 'Architecture'),
    'https://images.unsplash.com/photo-1515879218367-8466d910aaa4?auto=format&fit=crop&w=1200&q=80',
    'published',
    5,
    128,
    NOW()
  ),
  (
    'Vue 3 组合式 API 的页面组织',
    'vue3-composition-page-structure',
    '用组合式 API 管理列表、详情和编辑表单，让页面保持简洁可读。',
    'Vue 3 的组合式 API 很适合把状态、请求和派生数据收拢在同一段逻辑里。对于技术博客前台，列表页重点是搜索、分类和分页；详情页重点是内容阅读体验；管理页重点是表单状态与提交反馈。\n\n当页面复杂度上升时，可以再把 API 请求、表单校验和筛选状态抽成 composable。',
    'Frontend',
    JSON_ARRAY('Vue3', 'Composition API', 'UI'),
    'https://images.unsplash.com/photo-1555066931-4365d14bab8c?auto=format&fit=crop&w=1200&q=80',
    'published',
    4,
    96,
    NOW()
  ),
  (
    'nginx 作为前端入口和 API 网关',
    'nginx-static-and-api-gateway',
    '一个 nginx 容器同时处理 Vue 静态资源、SPA fallback 和 Go API 代理。',
    '在小型团队或个人项目里，让 nginx 统一承接浏览器流量是很自然的选择。它可以托管构建后的前端资源，并把 /api 请求转发给 Go 服务。\n\n这样部署后，前端不需要知道后端容器地址，跨域问题也会少很多。后续要加缓存、压缩、限流和访问日志，也能集中在入口层完成。',
    'DevOps',
    JSON_ARRAY('nginx', 'Docker', 'Deploy'),
    'https://images.unsplash.com/photo-1451187580459-43490279c0fa?auto=format&fit=crop&w=1200&q=80',
    'published',
    3,
    152,
    NOW()
  );
