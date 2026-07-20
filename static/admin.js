// RedChef - Admin Client

const API = {
  login: '/api/admin/login',
  logout: '/api/admin/logout',
  posts: '/api/admin/posts',
  upload: '/api/admin/upload',
  users: '/api/admin/users',
  comments: '/api/admin/comments',
  settings: '/api/admin/settings/analytics',
  emailSettings: '/api/admin/settings/email',
  setupStatus: '/api/setup/status',
  postsSimple: '/api/admin/posts/simple',
};

// Check existing auth on load
async function checkAuth() {
  try {
    const setupRes = await fetch(API.setupStatus);
    if (setupRes.ok) {
      const status = await setupRes.json();
      if (status.needs_setup) {
        window.location.href = '/setup';
        return;
      }
    }
  } catch (_) {}

  try {
    const res = await fetch(API.posts);
    if (res.ok) {
      showDashboard();
      return;
    }
  } catch (_) {}
  showLogin();
}

// Login
async function handleLogin(e) {
  e.preventDefault();
  const email = document.getElementById('username').value;
  const password = document.getElementById('password').value;
  const error = document.getElementById('login-error');

  try {
    const res = await fetch(API.login, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });

    if (!res.ok) {
      const data = await res.json();
      error.textContent = data.error || 'Login failed';
      return;
    }

    showDashboard();
  } catch (err) {
    error.textContent = 'Network error';
  }
}

// Logout
async function handleLogout() {
  await fetch(API.logout, { method: 'POST' });
  showLogin();
}

function showLogin() {
  document.getElementById('login-section').style.display = 'flex';
  document.getElementById('dashboard-section').style.display = 'none';
}

function showDashboard() {
  document.getElementById('login-section').style.display = 'none';
  document.getElementById('dashboard-section').style.display = 'block';
  loadPosts();
  loadUsers();
  loadComments();
  loadSettings();
  loadEmailSettings();
}

// Upload
function handleUpload(e) {
  e.preventDefault();
  const form = e.target;
  const formData = new FormData(form);
  const status = document.getElementById('upload-status');
  const progress = document.getElementById('upload-progress');
  const progressFill = document.getElementById('progress-fill');
  const progressText = document.getElementById('progress-text');
  const uploadBtn = document.getElementById('upload-btn');

  status.textContent = '';
  progress.style.display = 'flex';
  progressFill.style.width = '0%';
  progressText.textContent = 'Starting...';
  uploadBtn.disabled = true;

  const xhr = new XMLHttpRequest();

  xhr.upload.onprogress = function (e) {
    if (e.lengthComputable) {
      const pct = Math.round((e.loaded / e.total) * 100);
      progressFill.style.width = pct + '%';
      progressText.textContent = pct + '%';
    } else {
      progressText.textContent = 'Uploading...';
    }
  };

  xhr.onload = function () {
    uploadBtn.disabled = false;
    if (xhr.status >= 200 && xhr.status < 300) {
      progressFill.style.width = '100%';
      progressText.textContent = '✅ Done!';
      status.textContent = 'Upload accepted! Processing in background.';
      status.style.color = '#4CAF50';
      form.reset();
      setTimeout(() => { progress.style.display = 'none'; }, 1500);
      loadPosts();
    } else {
      progress.style.display = 'none';
      try {
        const data = JSON.parse(xhr.responseText);
        status.textContent = data.error || 'Upload failed';
      } catch (_) {
        status.textContent = 'Upload failed';
      }
      status.style.color = '#D42B2B';
    }
  };

  xhr.onerror = function () {
    uploadBtn.disabled = false;
    progress.style.display = 'none';
    status.textContent = 'Network error';
    status.style.color = '#D42B2B';
  };

  xhr.open('POST', API.upload, true);
  xhr.send(formData);
}

// Load posts
let allPosts = [];

async function loadPosts() {
  const tbody = document.getElementById('posts-table-body');
  tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:2rem;color:#888;">Loading...</td></tr>';

  try {
    const res = await fetch(API.posts);
    if (!res.ok) throw new Error('Failed');
    const posts = await res.json();
    allPosts = posts;

    if (posts.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:2rem;color:#888;">No posts yet. Upload something!</td></tr>';
      return;
    }

    tbody.innerHTML = posts.map(post => {
      const thumbnail = post.thumbnail || post.filename;
      return `
      <tr>
        <td>
          ${post.media_type === 'video'
            ? `<video src="/uploads/${thumbnail}" style="width:80px;height:50px;object-fit:cover;border-radius:4px;" preload="metadata"></video>`
            : `<img src="/uploads/${thumbnail}" style="width:80px;height:50px;object-fit:cover;border-radius:4px;" loading="lazy">`
          }
          ${post.processing ? '<span style="font-size:0.7rem;color:var(--gold);display:block;">⏳ processing</span>' : ''}
        </td>
        <td>${escapeHtml(post.title)}</td>
        <td><span class="type-badge type-${post.media_type}">${post.media_type}</span></td>
        <td>
          <span class="lock-status">${post.locked ? '🔒' : '🔓'}</span>
          <button class="lock-btn" onclick="toggleLock(${post.id}, ${post.locked})">
            ${post.locked ? 'Unlock' : 'Lock'}
          </button>
        </td>
        <td>${new Date(post.created_at).toLocaleDateString()}</td>
        <td>
          <button class="lock-btn" onclick="openEditModal(${post.id})">✏️ Edit</button>
          <button class="lock-btn" onclick="openLinksModal(${post.id})">🔗 Links</button>
          <button class="delete-btn" onclick="deletePost(${post.id})">Delete</button>
        </td>
      </tr>
    `}).join('');
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:2rem;color:#D42B2B;">Failed to load posts</td></tr>';
  }
}

// Toggle lock state
async function toggleLock(id, currentlyLocked) {
  const newLocked = !currentlyLocked;
  try {
    const res = await fetch(`${API.posts}/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ locked: newLocked }),
    });
    if (!res.ok) throw new Error('Failed');
    loadPosts();
  } catch (err) {
    alert('Failed to update lock state');
  }
}

// Delete post
async function deletePost(id) {
  if (!confirm('Delete this post? This cannot be undone.')) return;

  try {
    const res = await fetch(`${API.posts}/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed');
    loadPosts();
  } catch (err) {
    alert('Failed to delete post');
  }
}

// ── Edit Post ──

let currentEditPostId = null;

function openEditModal(postId) {
  const post = allPosts.find(p => p.id === postId);
  if (!post) return;
  currentEditPostId = postId;
  document.getElementById('edit-title').value = post.title || '';
  document.getElementById('edit-description').value = post.description || '';
  document.getElementById('edit-modal').style.display = 'flex';
}

function closeEditModal() {
  document.getElementById('edit-modal').style.display = 'none';
  currentEditPostId = null;
}

async function handleEditSubmit(e) {
  e.preventDefault();
  if (!currentEditPostId) return;

  const title = document.getElementById('edit-title').value.trim();
  const description = document.getElementById('edit-description').value;
  if (!title) return;

  try {
    const res = await fetch(`${API.posts}/${currentEditPostId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, description }),
    });
    if (!res.ok) {
      const data = await res.json();
      showToast('❌ ' + (data.error || 'Failed to update post'));
      return;
    }
    showToast('✅ Post updated');
    closeEditModal();
    loadPosts();
  } catch (_) {
    showToast('❌ Failed to update post');
  }
}

// ── Comments Management ──

async function loadComments() {
  const tbody = document.getElementById('comments-table-body');
  tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:2rem;color:#888;">Loading...</td></tr>';

  try {
    const res = await fetch(API.comments);
    if (!res.ok) throw new Error('Failed');
    const comments = await res.json();

    if (comments.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:2rem;color:#888;">No comments yet.</td></tr>';
      return;
    }

    tbody.innerHTML = comments.map(c => `
      <tr>
        <td title="Post #${c.post_id}">${escapeHtml(c.post_title || ('Post #' + c.post_id))}</td>
        <td>${escapeHtml(c.username || 'User')}</td>
        <td style="max-width:320px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${escapeHtml(c.body)}">${escapeHtml(c.body)}</td>
        <td>${new Date(c.created_at).toLocaleDateString()}</td>
        <td>
          <button class="delete-btn" onclick="deleteComment(${c.id})">Delete</button>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:2rem;color:#D42B2B;">Failed to load comments</td></tr>';
  }
}

async function deleteComment(id) {
  if (!confirm('Delete this comment?')) return;

  try {
    const res = await fetch(`${API.comments}/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed');
    showToast('🗑️ Comment deleted');
    loadComments();
  } catch (err) {
    showToast('❌ Failed to delete comment');
  }
}

// ── Users Management ──

async function loadUsers() {
  const tbody = document.getElementById('users-table-body');
  tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:2rem;color:#888;">Loading...</td></tr>';

  try {
    const res = await fetch(API.users);
    if (!res.ok) throw new Error('Failed');
    const users = await res.json();

    if (users.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:2rem;color:#888;">No users</td></tr>';
      return;
    }

    tbody.innerHTML = users.map(user => `
      <tr>
        <td>${escapeHtml(user.email)}</td>
        <td><span class="type-badge ${user.role === 'admin' ? 'type-photo' : 'type-video'}">${user.role}</span></td>
        <td>
          <span class="lock-status">${user.paid ? '✅ Paid' : '❌ Not Paid'}</span>
          <button class="lock-btn" onclick="togglePaid(${user.id}, ${user.paid})">
            ${user.paid ? 'Revoke' : 'Grant'}
          </button>
        </td>
        <td>${new Date(user.created_at).toLocaleDateString()}</td>
        <td>
          ${user.role !== 'admin'
            ? `<button class="delete-btn" onclick="deleteUser(${user.id})">Delete</button>`
            : '<span style="color:#888;font-size:0.75rem;">Admin</span>'
          }
        </td>
      </tr>
    `).join('');
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;padding:2rem;color:#D42B2B;">Failed to load users</td></tr>';
  }
}

async function togglePaid(id, currentlyPaid) {
  try {
    const res = await fetch(`${API.users}/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paid: !currentlyPaid }),
    });
    if (!res.ok) throw new Error('Failed');
    loadUsers();
  } catch (err) {
    alert('Failed to update user');
  }
}

async function deleteUser(id) {
  if (!confirm('Delete this user? This cannot be undone.')) return;

  try {
    const res = await fetch(`${API.users}/${id}`, { method: 'DELETE' });
    if (!res.ok) {
      const data = await res.json();
      alert(data.error || 'Failed to delete user');
      return;
    }
    loadUsers();
  } catch (err) {
    alert('Failed to delete user');
  }
}

// Settings
async function loadSettings() {
  try {
    const res = await fetch(API.settings);
    if (!res.ok) return;
    const settings = await res.json();
    document.getElementById('umami-script-url').value = settings.umami_script_url || '';
    document.getElementById('umami-website-id').value = settings.umami_website_id || '';
    document.getElementById('tracking-enabled').checked = settings.tracking_enabled || false;
  } catch (_) {}
}

async function handleSaveSettings(e) {
  e.preventDefault();
  const status = document.getElementById('settings-status');
  status.textContent = 'Saving...';
  status.style.color = '#F5C518';

  try {
    const res = await fetch(API.settings, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        umami_script_url: document.getElementById('umami-script-url').value,
        umami_website_id: document.getElementById('umami-website-id').value,
        tracking_enabled: document.getElementById('tracking-enabled').checked,
      }),
    });

    if (!res.ok) {
      const data = await res.json();
      status.textContent = data.error || 'Failed to save settings';
      status.style.color = '#D42B2B';
      return;
    }

    status.textContent = 'Settings saved';
    status.style.color = '#4CAF50';
  } catch (err) {
    status.textContent = 'Network error';
    status.style.color = '#D42B2B';
  }
}

// Email Settings
async function loadEmailSettings() {
  try {
    const res = await fetch(API.emailSettings);
    if (!res.ok) return;
    const s = await res.json();
    document.getElementById('smtp-host').value = s.smtp_host || '';
    document.getElementById('smtp-port').value = s.smtp_port || 587;
    document.getElementById('smtp-username').value = s.username || '';
    document.getElementById('smtp-password').value = s.password || '';
    document.getElementById('smtp-from').value = s.from_addr || '';
    document.getElementById('smtp-encryption').value = s.encryption || 'tls';
  } catch (_) {}
}

async function handleSaveEmailSettings(e) {
  e.preventDefault();
  const status = document.getElementById('email-settings-status');
  status.textContent = 'Saving...';
  status.style.color = '#F5C518';

  try {
    const res = await fetch(API.emailSettings, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        smtp_host: document.getElementById('smtp-host').value,
        smtp_port: parseInt(document.getElementById('smtp-port').value) || 587,
        username: document.getElementById('smtp-username').value,
        password: document.getElementById('smtp-password').value,
        from_addr: document.getElementById('smtp-from').value,
        encryption: document.getElementById('smtp-encryption').value,
      }),
    });

    if (!res.ok) {
      const data = await res.json();
      status.textContent = data.error || 'Failed to save email settings';
      status.style.color = '#D42B2B';
      return;
    }

    status.textContent = 'Email settings saved';
    status.style.color = '#4CAF50';
  } catch (err) {
    status.textContent = 'Network error';
    status.style.color = '#D42B2B';
  }
}

// Umami tracking init
async function initUmamiTracking() {
  try {
    const res = await fetch('/api/settings/analytics');
    if (!res.ok) return;
    const settings = await res.json();
    if (!settings.tracking_enabled || !settings.umami_script_url || !settings.umami_website_id) return;

    let src = settings.umami_script_url;
    if (src && !src.match(/\.js$/)) {
      src = src.replace(/\/+$/, '') + '/script.js';
    }
    const script = document.createElement('script');
    script.async = true;
    script.defer = true;
    script.src = src;
    script.setAttribute('data-website-id', settings.umami_website_id);
    document.head.appendChild(script);
  } catch (_) {}
}

// ── Linked Posts Management ──

let currentLinksPostId = null;
let allPostsForLinks = [];

async function openLinksModal(postId) {
  currentLinksPostId = postId;
  const modal = document.getElementById('links-modal');
  modal.style.display = 'flex';

  // Load all posts for the picker
  try {
    const res = await fetch(API.postsSimple);
    if (!res.ok) throw new Error('Failed');
    allPostsForLinks = await res.json();
  } catch (_) {
    allPostsForLinks = [];
  }

  // Load existing links for this post
  let linkedIds = [];
  try {
    const res = await fetch('/api/posts/' + postId);
    if (res.ok) {
      const post = await res.json();
      if (post.linked_posts) {
        linkedIds = post.linked_posts.map(lp => lp.linked_post_id);
      }
    }
  } catch (_) {}

  const list = document.getElementById('links-list');
  list.innerHTML = allPostsForLinks
    .filter(p => p.id !== postId)
    .map(p => {
      const checked = linkedIds.includes(p.id) ? 'checked' : '';
      return `
        <label class="links-item">
          <input type="checkbox" value="${p.id}" ${checked}>
          <span>${escapeHtml(p.title)} (ID: ${p.id})</span>
        </label>
      `;
    }).join('') || '<p style="color:#888;">No other posts available.</p>';
}

function closeLinksModal() {
  document.getElementById('links-modal').style.display = 'none';
  currentLinksPostId = null;
}

async function saveLinks() {
  if (!currentLinksPostId) return;
  const checkboxes = document.querySelectorAll('#links-list input[type="checkbox"]:checked');
  const linkedIds = Array.from(checkboxes).map(cb => parseInt(cb.value));

  try {
    const res = await fetch('/api/admin/posts/' + currentLinksPostId + '/links', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ linked_ids: linkedIds }),
    });
    if (!res.ok) throw new Error('Failed');
    showToast('🔗 Links saved');
    closeLinksModal();
    loadPosts();
  } catch (_) {
    showToast('❌ Failed to save links');
  }
}

function showToast(msg) {
  const el = document.getElementById('toast-admin') || (() => {
    const t = document.createElement('div');
    t.id = 'toast-admin';
    t.style.cssText = 'position:fixed;bottom:20px;right:20px;background:rgba(0,0,0,0.85);color:#fff;padding:12px 20px;border-radius:8px;font-size:0.9rem;z-index:9999;transition:opacity 0.3s;';
    document.body.appendChild(t);
    return t;
  })();
  el.textContent = msg;
  el.style.opacity = '1';
  setTimeout(() => { el.style.opacity = '0'; }, 3000);
}

// Helpers
function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// Init
document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('login-form').addEventListener('submit', handleLogin);
  document.getElementById('upload-form').addEventListener('submit', handleUpload);
  document.getElementById('settings-form').addEventListener('submit', handleSaveSettings);
  document.getElementById('email-settings-form').addEventListener('submit', handleSaveEmailSettings);
  document.getElementById('edit-form').addEventListener('submit', handleEditSubmit);
  document.getElementById('logout-btn').addEventListener('click', handleLogout);
  initUmamiTracking();
  checkAuth();
});
