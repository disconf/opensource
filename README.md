# Tech Blog

一个简洁完整的技术博客项目，采用 `nginx + Go + Vue 3 + MySQL` 架构。

## 功能

- 文章列表、搜索、分类筛选、标签展示
- 文章详情页，支持摘要、正文、阅读时间和浏览量
- 用户注册、登录、随机验证码
- 普通用户只能管理自己的文章，管理员可以管理全部文章
- 登录用户可以创建、编辑、发布/草稿、删除文章
- 文章正文支持 Markdown 编辑和预览
- 封面图、正文图片支持本地上传
- Go REST API，MySQL 持久化
- nginx 静态资源托管与 `/api` 反向代理
- 留言板提交、公开展示与管理员审核
- 管理后台概览、留言管理、站点及 SEO 配置
- 管理员查看注册信息、调整用户角色和删除账号
- 登录与注册均使用一次性随机验证码，登录框不预填账号密码
- 搜索防抖、动态页面标题与 Open Graph 元信息

## 快速启动

```bash
docker compose up --build
```

打开：

- 博客前台：http://localhost
- 后端 API：http://localhost/api/posts

默认管理员在 `docker-compose.yml` 中配置。前端管理入口右上角 `写作台`，默认账号：

```text
username: admin
password: change-me
```

## 本地开发

初始化外部 MySQL：

```bash
cd backend
MYSQL_ROOT_DSN='root:your_password@tcp(192.168.85.133:3306)/?parseTime=true&charset=utf8mb4&multiStatements=true&loc=Local' \
MYSQL_DATABASE='tech_blog' \
go run ./cmd/mysqlcheck -setup
```

后端：

```bash
cd backend
go mod tidy
ADMIN_USERNAME='admin' \
ADMIN_PASSWORD='change-me' \
MYSQL_DSN='root:your_password@tcp(192.168.85.133:3306)/tech_blog?parseTime=true&charset=utf8mb4&loc=Local' \
go run .
```

前端：

```bash
cd frontend
npm install
npm run dev
```

## API

- `GET /api/health`
- `GET /api/posts?q=&category=`
- `GET /api/posts/{slug}`
- `GET /api/comments`
- `POST /api/comments`
- `GET /api/settings`
- `GET /api/captcha`
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/admin/posts`
- `POST /api/admin/posts`
- `PUT /api/admin/posts/{id}`
- `DELETE /api/admin/posts/{id}`
- `GET /api/admin/comments`
- `PUT /api/admin/comments/{id}`
- `DELETE /api/admin/comments/{id}`
- `GET /api/admin/stats`
- `GET /api/admin/users`
- `PUT /api/admin/users/{id}`
- `DELETE /api/admin/users/{id}`
- `PUT /api/admin/settings`
- `POST /api/uploads`

管理接口需要登录后的 Header：

```text
Authorization: Bearer <token>
```
