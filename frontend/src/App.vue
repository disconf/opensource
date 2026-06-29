<template>
  <div class="app-shell">
    <header class="topbar">
      <button class="brand" type="button" @click="goHome" aria-label="返回首页">
        <span class="brand-mark">TB</span>
        <span>
          <strong>{{ settings.siteName }}</strong>
          <small>{{ currentUser ? `${currentUser.username} / ${currentUser.role}` : settings.siteSubtitle }}</small>
        </span>
      </button>
      <nav>
        <button :class="{ active: view === 'home' }" @click="goHome">文章</button>
        <button :class="{ active: view === 'guestbook' }" @click="openGuestbook">
          <MessageSquare :size="16" /> 留言板
        </button>
        <button :class="{ active: view === 'admin' }" @click="openAdmin">
          <Shield :size="16" />
          写作台
        </button>
        <button v-if="currentUser" @click="logout">
          <LogOut :size="16" />
          退出
        </button>
      </nav>
    </header>

    <main v-if="view === 'home'" class="layout">
      <aside class="sidebar">
        <div class="search">
          <Search :size="18" />
          <input v-model="filters.q" placeholder="搜索标题、摘要或正文" @input="queueSearch" />
        </div>
        <section>
          <h2>分类</h2>
          <button class="category" :class="{ active: !filters.category }" @click="selectCategory('')">
            全部
            <span>{{ posts.length }}</span>
          </button>
          <button
            v-for="item in categoryList"
            :key="item.name"
            class="category"
            :class="{ active: filters.category === item.name }"
            @click="selectCategory(item.name)"
          >
            {{ item.name }}
            <span>{{ item.count }}</span>
          </button>
        </section>
      </aside>

      <section class="content-list">
        <div class="section-head">
          <div>
            <p>Latest Articles</p>
            <h1>技术笔记</h1>
          </div>
          <button class="icon-button" type="button" title="刷新" @click="loadPosts">
            <RefreshCw :size="18" />
          </button>
        </div>

        <article v-for="item in posts" :key="item.id" class="post-card" @click="openPost(item.slug)">
          <img :src="item.coverUrl || fallbackCover" :alt="item.title" />
          <div>
            <div class="meta">
              <span>{{ item.category }}</span>
              <span>{{ item.authorName || 'Admin' }}</span>
              <span>{{ item.readMinutes }} min</span>
              <span>{{ item.views }} views</span>
            </div>
            <h2>{{ item.title }}</h2>
            <p>{{ item.summary }}</p>
            <div class="tags">
              <span v-for="tag in item.tags" :key="tag">{{ tag }}</span>
            </div>
          </div>
        </article>

        <p v-if="!loading && posts.length === 0" class="empty">暂无匹配文章</p>
      </section>
    </main>

    <main v-if="view === 'detail'" class="reader">
      <button class="back-button" @click="goHome">
        <ArrowLeft :size="18" />
        返回
      </button>
      <article v-if="activePost">
        <img class="reader-cover" :src="activePost.coverUrl || fallbackCover" :alt="activePost.title" />
        <div class="meta">
          <span>{{ activePost.category }}</span>
          <span>{{ activePost.authorName || 'Admin' }}</span>
          <span>{{ activePost.readMinutes }} min</span>
          <span>{{ activePost.views }} views</span>
        </div>
        <h1>{{ activePost.title }}</h1>
        <p class="summary">{{ activePost.summary }}</p>
        <div class="markdown-body" v-html="renderMarkdown(activePost.content)"></div>
      </article>
    </main>

    <main v-if="view === 'guestbook'" class="guestbook">
      <section class="guestbook-intro">
        <p>Guestbook</p>
        <h1>留下你的想法</h1>
        <p>问题、建议，或者只是路过打个招呼。新留言会在审核后公开展示。</p>
      </section>
      <form class="comment-form" @submit.prevent="submitComment">
        <div class="form-grid">
          <label>称呼<input v-model="commentForm.name" maxlength="60" required /></label>
          <label>邮箱（不会公开）<input v-model="commentForm.email" type="email" maxlength="120" /></label>
        </div>
        <label>留言<textarea v-model="commentForm.content" rows="5" maxlength="1000" required /></label>
        <div class="comment-submit">
          <span class="message">{{ commentMessage }}</span>
          <button class="primary" type="submit"><Send :size="16" /> 提交留言</button>
        </div>
      </form>
      <section class="comment-list">
        <article v-for="item in comments" :key="item.id" class="comment-card">
          <div class="comment-avatar">{{ item.name.slice(0, 1).toUpperCase() }}</div>
          <div><div class="comment-meta"><strong>{{ item.name }}</strong><time>{{ formatDate(item.createdAt) }}</time></div><p>{{ item.content }}</p></div>
        </article>
        <p v-if="comments.length === 0" class="empty">还没有公开留言，来坐第一排吧。</p>
      </section>
    </main>

    <main v-if="view === 'admin'" class="admin">
      <section v-if="!currentUser" class="auth-panel">
        <div class="section-head compact">
          <div>
            <p>Account</p>
            <h1>{{ authMode === 'login' ? '登录' : '注册' }}</h1>
          </div>
          <button class="icon-button" type="button" title="刷新验证码" @click="loadCaptcha">
            <RefreshCw :size="18" />
          </button>
        </div>

        <div class="segmented">
          <button :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
          <button :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
        </div>

        <form v-if="authMode === 'login'" class="auth-form" autocomplete="off" @submit.prevent="login">
          <label>
            用户名
            <input v-model="loginForm.username" name="login-account" autocomplete="off" required />
          </label>
          <label>
            密码
            <input v-model="loginForm.password" name="login-secret" type="password" autocomplete="new-password" required />
          </label>
          <label>
            随机验证码
            <span class="captcha-line">
              <input v-model="loginForm.captcha" name="login-captcha" autocomplete="off" maxlength="8" required />
              <button class="captcha-code" type="button" title="换一个验证码" @click="loadCaptcha">{{ captcha.code }}</button>
            </span>
          </label>
          <button class="primary" type="submit">
            <LogIn :size="16" />
            登录
          </button>
        </form>

        <form v-else class="auth-form" @submit.prevent="register">
          <div class="form-grid">
            <label>
              用户名
              <input v-model="registerForm.username" required />
            </label>
            <label>
              密码
              <input v-model="registerForm.password" type="password" minlength="6" required />
            </label>
            <label>
              手机号
              <input v-model="registerForm.phone" required />
            </label>
            <label>
              邮箱
              <input v-model="registerForm.email" type="email" required />
            </label>
          </div>
          <label>
            验证码
            <span class="captcha-line">
              <input v-model="registerForm.captcha" required />
              <strong>{{ captcha.code }}</strong>
            </span>
          </label>
          <button class="primary" type="submit">
            <UserPlus :size="16" />
            注册并登录
          </button>
        </form>
        <p v-if="message" class="message">{{ message }}</p>
      </section>

      <template v-else>
        <section class="admin-list">
          <div v-if="currentUser.role === 'admin'" class="admin-tabs">
            <button :class="{ active: adminTab === 'posts' }" @click="adminTab = 'posts'"><FileText :size="16" />文章</button>
            <button :class="{ active: adminTab === 'comments' }" @click="openCommentAdmin"><MessageSquare :size="16" />留言</button>
            <button :class="{ active: adminTab === 'users' }" @click="openUserAdmin"><Users :size="16" />注册用户</button>
            <button :class="{ active: adminTab === 'settings' }" @click="adminTab = 'settings'"><Settings :size="16" />站点设置</button>
          </div>
          <div v-if="currentUser.role === 'admin'" class="stat-grid">
            <span><strong>{{ stats.published }}</strong>已发布</span><span><strong>{{ stats.pendingComments }}</strong>待审核</span>
          </div>
          <template v-if="adminTab === 'posts'">
          <div class="section-head compact">
            <div>
              <p>{{ currentUser.role === 'admin' ? 'All Posts' : 'My Posts' }}</p>
              <h1>文章管理</h1>
            </div>
            <button class="primary" @click="newPost">
              <Plus :size="16" />
              新建
            </button>
          </div>
          <button
            v-for="item in adminPosts"
            :key="item.id"
            class="admin-row"
            :class="{ active: form.id === item.id }"
            @click="editPost(item)"
          >
            <span>
              <strong>{{ item.title }}</strong>
              <small>{{ item.authorName || 'Admin' }} / {{ item.category }} / {{ statusLabel(item.status) }}</small>
            </span>
            <Edit3 :size="15" />
          </button>
          <p v-if="adminPosts.length === 0" class="empty">还没有文章</p>
          </template>
        </section>

        <form v-if="adminTab === 'posts'" class="editor" @submit.prevent="savePost">
          <div class="form-grid">
            <label>
              标题
              <input v-model="form.title" required />
            </label>
            <label>
              Slug
              <input v-model="form.slug" required />
            </label>
            <label>
              分类
              <input v-model="form.category" required />
            </label>
            <label>
              标签
              <input v-model="tagText" placeholder="Go, Vue3, nginx" />
            </label>
            <label>
              封面图
              <span class="upload-line">
                <input v-model="form.coverUrl" placeholder="https://... 或上传图片" />
                <button class="icon-button" type="button" title="上传封面" @click="coverPicker?.click()">
                  <ImagePlus :size="18" />
                </button>
              </span>
              <input ref="coverPicker" class="file-input" type="file" accept="image/*" @change="uploadCover" />
            </label>
            <label>
              阅读时间
              <input v-model.number="form.readMinutes" type="number" min="1" />
            </label>
          </div>

          <img v-if="form.coverUrl" class="cover-preview" :src="form.coverUrl" alt="封面预览" />

          <label>
            摘要
            <textarea v-model="form.summary" rows="3" required />
          </label>

          <div class="editor-head">
            <span>正文 Markdown</span>
            <div class="editor-tools">
              <button class="icon-button" type="button" title="插入图片" @click="contentPicker?.click()">
                <ImagePlus :size="18" />
              </button>
              <button class="icon-button" type="button" title="切换预览" @click="previewMarkdown = !previewMarkdown">
                <Eye :size="18" />
              </button>
            </div>
          </div>
          <textarea
            v-if="!previewMarkdown"
            ref="contentInput"
            v-model="form.content"
            class="markdown-editor"
            rows="16"
            placeholder="# 标题&#10;&#10;这里可以写 **加粗**、列表、代码块和图片。"
            required
          />
          <div v-else class="markdown-preview markdown-body" v-html="renderMarkdown(form.content)"></div>
          <input ref="contentPicker" class="file-input" type="file" accept="image/*" @change="uploadContentImage" />

          <div class="editor-actions">
            <select v-model="form.status">
              <option value="draft">草稿</option>
              <option value="published">发布</option>
            </select>
            <button class="danger" type="button" :disabled="!form.id" @click="deletePost">
              <Trash2 :size="16" />
              删除
            </button>
            <button class="primary" type="submit">
              <Save :size="16" />
              保存
            </button>
          </div>
          <p v-if="message" class="message">{{ message }}</p>
        </form>

        <section v-else-if="adminTab === 'comments'" class="editor moderation-panel">
          <div class="section-head compact"><div><p>Moderation</p><h1>留言审核</h1></div></div>
          <article v-for="item in adminComments" :key="item.id" class="moderation-row">
            <div><div class="comment-meta"><strong>{{ item.name }}</strong><span :class="['status-pill', item.status]">{{ item.status === 'approved' ? '已公开' : '待审核' }}</span></div><small>{{ item.email || '未留邮箱' }} · {{ formatDate(item.createdAt) }}</small><p>{{ item.content }}</p></div>
            <div class="row-actions"><button v-if="item.status === 'pending'" class="primary" @click="moderateComment(item, 'approved')">通过</button><button class="danger" @click="removeComment(item)">删除</button></div>
          </article>
          <p v-if="adminComments.length === 0" class="empty">暂时没有留言</p>
        </section>

        <form v-else-if="adminTab === 'settings'" class="editor settings-panel" @submit.prevent="saveSettings">
          <div class="section-head compact"><div><p>Site & SEO</p><h1>站点设置</h1></div></div>
          <div class="form-grid"><label>站点名称<input v-model="settings.siteName" required /></label><label>副标题<input v-model="settings.siteSubtitle" /></label><label>站点地址<input v-model="settings.siteUrl" placeholder="https://example.com" /></label><label>默认分享图<input v-model="settings.defaultOgImage" placeholder="https://..." /></label></div>
          <label>SEO 描述<textarea v-model="settings.seoDescription" rows="3" maxlength="200" /></label>
          <label>SEO 关键词<input v-model="settings.seoKeywords" placeholder="Go, Vue, 技术博客" /></label>
          <label>页脚文字<input v-model="settings.footerText" /></label>
          <div class="editor-actions"><button class="primary" type="submit"><Save :size="16" />保存配置</button></div>
          <p v-if="message" class="message">{{ message }}</p>
        </form>

        <section v-else class="editor user-panel">
          <div class="section-head compact"><div><p>Registered Accounts</p><h1>注册用户</h1></div><span class="message">共 {{ adminUsers.length }} 人</span></div>
          <div class="user-table-wrap">
            <table class="user-table">
              <thead><tr><th>用户</th><th>联系方式</th><th>角色</th><th>文章</th><th>注册时间</th><th>操作</th></tr></thead>
              <tbody><tr v-for="item in adminUsers" :key="item.id"><td><strong>{{ item.username }}</strong><small>#{{ item.id }}</small></td><td><span>{{ item.phone }}</span><small>{{ item.email }}</small></td><td><select :value="item.role" :disabled="item.id === currentUser.id" @change="changeUserRole(item, $event.target.value)"><option value="user">普通用户</option><option value="admin">管理员</option></select></td><td>{{ item.postCount }}</td><td>{{ formatDate(item.createdAt) }}</td><td><button class="danger" :disabled="item.id === currentUser.id" @click="removeUser(item)"><Trash2 :size="15" />删除</button></td></tr></tbody>
            </table>
          </div>
          <p v-if="adminUsers.length === 0" class="empty">暂无注册用户</p>
        </section>
      </template>
    </main>
    <footer>{{ settings.footerText || '持续记录，认真构建。' }}</footer>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import {
  ArrowLeft,
  Edit3,
  Eye,
  ImagePlus,
  LogIn,
  LogOut,
  FileText,
  MessageSquare,
  Plus,
  RefreshCw,
  Save,
  Search,
  Send,
  Settings,
  Shield,
  Trash2,
  Users,
  UserPlus
} from '@lucide/vue'

const fallbackCover =
  'https://images.unsplash.com/photo-1498050108023-c5249f4df085?auto=format&fit=crop&w=1200&q=80'

const view = ref('home')
const loading = ref(false)
const posts = ref([])
const adminPosts = ref([])
const comments = ref([])
const adminComments = ref([])
const adminUsers = ref([])
const adminTab = ref('posts')
const activePost = ref(null)
const categories = ref({})
const message = ref('')
const token = ref(localStorage.getItem('authToken') || '')
const currentUser = ref(null)
const authMode = ref('login')
const previewMarkdown = ref(false)
const tagText = ref('')
const coverPicker = ref(null)
const contentPicker = ref(null)
const contentInput = ref(null)
const captcha = reactive({ id: '', code: '' })
const filters = reactive({ q: '', category: '' })
const commentForm = reactive({ name: '', email: '', content: '' })
const commentMessage = ref('')
const stats = reactive({ posts: 0, published: 0, pendingComments: 0, users: 0 })
const settings = reactive({ siteName: 'Tech Blog', siteSubtitle: 'Engineering notes', siteUrl: '', seoDescription: '', seoKeywords: '', defaultOgImage: '', footerText: '' })
const loginForm = reactive({ username: '', password: '', captcha: '' })
const registerForm = reactive({
  username: '',
  password: '',
  phone: '',
  email: '',
  captcha: ''
})
const form = reactive(emptyForm())

const categoryList = computed(() =>
  Object.entries(categories.value)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => a.name.localeCompare(b.name))
)

onMounted(async () => {
  await loadSettings()
  await loadPosts()
  await restoreSession()
  await loadCaptcha()
})

let searchTimer
function queueSearch() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(loadPosts, 320)
}

function emptyForm() {
  return {
    id: null,
    title: '',
    slug: '',
    summary: '',
    content: '',
    category: 'Backend',
    tags: [],
    coverUrl: '',
    status: 'draft',
    readMinutes: 3
  }
}

async function restoreSession() {
  if (!token.value) return
  try {
    currentUser.value = await request('/api/auth/me', { headers: authHeaders() })
  } catch {
    token.value = ''
    localStorage.removeItem('authToken')
  }
}

async function loadCaptcha() {
  const data = await request('/api/captcha')
  captcha.id = data.id
  captcha.code = data.code
  registerForm.captcha = ''
  loginForm.captcha = ''
}

async function login() {
  try {
    const data = await request('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ ...loginForm, captchaId: captcha.id })
    })
    applyAuth(data)
    Object.assign(loginForm, { username: '', password: '', captcha: '' })
    message.value = '登录成功'
    await loadAdminPosts()
  } catch (error) {
    message.value = error.message
    await loadCaptcha()
  }
}

async function register() {
  const data = await request('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify({
      ...registerForm,
      captchaId: captcha.id
    })
  })
  applyAuth(data)
  message.value = '注册成功'
  await loadCaptcha()
  await loadAdminPosts()
}

async function logout() {
  if (token.value) {
    await request('/api/auth/logout', { method: 'POST', headers: authHeaders() }).catch(() => null)
  }
  token.value = ''
  currentUser.value = null
  localStorage.removeItem('authToken')
  adminPosts.value = []
  newPost()
  goHome()
}

function applyAuth(data) {
  token.value = data.token
  currentUser.value = data.user
  localStorage.setItem('authToken', token.value)
}

async function loadPosts() {
  loading.value = true
  const params = new URLSearchParams()
  if (filters.q) params.set('q', filters.q)
  if (filters.category) params.set('category', filters.category)
  try {
    const data = await request(`/api/posts?${params}`)
    posts.value = data.items
    categories.value = data.categories
  } finally {
    loading.value = false
  }
}

async function loadAdminPosts() {
  if (!currentUser.value) return
  const data = await request('/api/admin/posts', { headers: authHeaders() })
  adminPosts.value = data.items
}

async function loadSettings() {
  Object.assign(settings, await request('/api/settings'))
  applySEO()
}

function applySEO(post = null) {
  const title = post ? `${post.title} · ${settings.siteName}` : settings.siteName
  const description = post?.summary || settings.seoDescription
  document.title = title
  setMeta('description', description)
  setMeta('keywords', settings.seoKeywords)
  setMeta('og:title', title, 'property')
  setMeta('og:description', description, 'property')
  setMeta('og:image', post?.coverUrl || settings.defaultOgImage, 'property')
  setMeta('og:type', post ? 'article' : 'website', 'property')
}

function setMeta(name, content, attribute = 'name') {
  if (!content) return
  let el = document.head.querySelector(`meta[${attribute}="${name}"]`)
  if (!el) { el = document.createElement('meta'); el.setAttribute(attribute, name); document.head.appendChild(el) }
  el.setAttribute('content', content)
}

async function openPost(slug) {
  activePost.value = await request(`/api/posts/${slug}`)
  view.value = 'detail'
  applySEO(activePost.value)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function selectCategory(category) {
  filters.category = category
  loadPosts()
}

function goHome() {
  view.value = 'home'
  activePost.value = null
  applySEO()
}

async function openGuestbook() {
  view.value = 'guestbook'
  applySEO()
  const data = await request('/api/comments')
  comments.value = data.items
}

async function submitComment() {
  const data = await request('/api/comments', { method: 'POST', body: JSON.stringify(commentForm) })
  commentMessage.value = data.message
  Object.assign(commentForm, { name: '', email: '', content: '' })
}

async function openAdmin() {
  view.value = 'admin'
  message.value = ''
  if (currentUser.value) {
    await loadAdminPosts()
    if (currentUser.value.role === 'admin') Object.assign(stats, await request('/api/admin/stats', { headers: authHeaders() }))
  }
}

async function openCommentAdmin() {
  adminTab.value = 'comments'
  const data = await request('/api/admin/comments', { headers: authHeaders() })
  adminComments.value = data.items
}

async function openUserAdmin() {
  adminTab.value = 'users'
  const data = await request('/api/admin/users', { headers: authHeaders() })
  adminUsers.value = data.items
}

async function changeUserRole(item, role) {
  try {
    await request(`/api/admin/users/${item.id}`, { method: 'PUT', headers: authHeaders(), body: JSON.stringify({ role }) })
    item.role = role
    message.value = `已更新 ${item.username} 的角色`
  } catch (error) {
    message.value = error.message
    await openUserAdmin()
  }
}

async function removeUser(item) {
  if (!window.confirm(`确定删除用户“${item.username}”吗？其登录会话将失效，文章会保留。`)) return
  await request(`/api/admin/users/${item.id}`, { method: 'DELETE', headers: authHeaders() })
  message.value = `已删除用户 ${item.username}`
  await openUserAdmin()
  Object.assign(stats, await request('/api/admin/stats', { headers: authHeaders() }))
}

async function moderateComment(item, status) {
  await request(`/api/admin/comments/${item.id}`, { method: 'PUT', headers: authHeaders(), body: JSON.stringify({ status }) })
  await openCommentAdmin(); Object.assign(stats, await request('/api/admin/stats', { headers: authHeaders() }))
}

async function removeComment(item) {
  await request(`/api/admin/comments/${item.id}`, { method: 'DELETE', headers: authHeaders() })
  await openCommentAdmin(); Object.assign(stats, await request('/api/admin/stats', { headers: authHeaders() }))
}

async function saveSettings() {
  Object.assign(settings, await request('/api/admin/settings', { method: 'PUT', headers: authHeaders(), body: JSON.stringify(settings) }))
  message.value = '站点与 SEO 配置已保存'
  applySEO()
}

function formatDate(value) { return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }

function newPost() {
  Object.assign(form, emptyForm())
  tagText.value = ''
  previewMarkdown.value = false
}

function editPost(item) {
  Object.assign(form, { ...item })
  tagText.value = item.tags.join(', ')
  previewMarkdown.value = false
}

async function uploadCover(event) {
  const url = await uploadSelectedFile(event)
  if (url) {
    form.coverUrl = url
    message.value = '封面已上传'
  }
}

async function uploadContentImage(event) {
  const url = await uploadSelectedFile(event)
  if (!url) return
  const markdown = `\n\n![图片](${url})\n\n`
  const el = contentInput.value
  if (!el) {
    form.content += markdown
    return
  }
  const start = el.selectionStart ?? form.content.length
  const end = el.selectionEnd ?? form.content.length
  form.content = form.content.slice(0, start) + markdown + form.content.slice(end)
  await nextTick()
  el.focus()
  el.selectionStart = el.selectionEnd = start + markdown.length
  message.value = '图片已插入正文'
}

async function uploadSelectedFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return ''
  const data = new FormData()
  data.append('file', file)
  const res = await fetch('/api/uploads', {
    method: 'POST',
    headers: authHeaders(),
    body: data
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || '上传失败')
  }
  const result = await res.json()
  return result.url
}

async function savePost() {
  const payload = {
    ...form,
    tags: tagText.value
      .split(',')
      .map((tag) => tag.trim())
      .filter(Boolean)
  }
  const path = form.id ? `/api/admin/posts/${form.id}` : '/api/admin/posts'
  const method = form.id ? 'PUT' : 'POST'
  await request(path, {
    method,
    headers: authHeaders(),
    body: JSON.stringify(payload)
  })
  message.value = '已保存'
  await loadAdminPosts()
  await loadPosts()
}

async function deletePost() {
  if (!form.id) return
  await request(`/api/admin/posts/${form.id}`, {
    method: 'DELETE',
    headers: authHeaders()
  })
  message.value = '已删除'
  newPost()
  await loadAdminPosts()
  await loadPosts()
}

function statusLabel(status) {
  return status === 'published' ? '已发布' : '草稿'
}

function authHeaders() {
  return {
    Authorization: `Bearer ${token.value}`
  }
}

async function request(path, options = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `Request failed: ${res.status}`)
  }
  if (res.status === 204) return null
  return res.json()
}

function renderMarkdown(markdown = '') {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n')
  const html = []
  let paragraph = []
  let list = []
  let inCode = false
  let code = []

  const flushParagraph = () => {
    if (paragraph.length) {
      html.push(`<p>${inlineMarkdown(paragraph.join(' '))}</p>`)
      paragraph = []
    }
  }
  const flushList = () => {
    if (list.length) {
      html.push(`<ul>${list.map((item) => `<li>${inlineMarkdown(item)}</li>`).join('')}</ul>`)
      list = []
    }
  }

  for (const line of lines) {
    if (line.trim().startsWith('```')) {
      if (inCode) {
        html.push(`<pre><code>${escapeHtml(code.join('\n'))}</code></pre>`)
        code = []
        inCode = false
      } else {
        flushParagraph()
        flushList()
        inCode = true
      }
      continue
    }
    if (inCode) {
      code.push(line)
      continue
    }
    if (!line.trim()) {
      flushParagraph()
      flushList()
      continue
    }
    const heading = line.match(/^(#{1,3})\s+(.+)$/)
    if (heading) {
      flushParagraph()
      flushList()
      html.push(`<h${heading[1].length}>${inlineMarkdown(heading[2])}</h${heading[1].length}>`)
      continue
    }
    const item = line.match(/^[-*]\s+(.+)$/)
    if (item) {
      flushParagraph()
      list.push(item[1])
      continue
    }
    paragraph.push(line)
  }

  flushParagraph()
  flushList()
  if (inCode) {
    html.push(`<pre><code>${escapeHtml(code.join('\n'))}</code></pre>`)
  }
  return html.join('')
}

function inlineMarkdown(text) {
  return escapeHtml(text)
    .replace(/!\[([^\]]*)\]\((https?:\/\/[^)\s]+|\/[^)\s]+)\)/g, '<img src="$2" alt="$1" />')
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+|\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}
</script>
