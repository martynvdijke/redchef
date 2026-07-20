// RedChef - Admin Client

const API = {
  login: '/api/admin/login',
  logout: '/api/admin/logout',
  posts: '/api/admin/posts',
  upload: '/api/admin/upload',
  users: '/api/admin/users',
  settings: '/api/admin/settings/analytics',
  setupStatus: '/api/setup/status',
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
  loadSettings();
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
async function loadPosts() {
  const tbody = document.getElementById('posts-table-body');
  tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:2rem;color:#888;">Loading...</td></tr>';

  try {
    const res = await fetch(API.posts);
    if (!res.ok) throw new Error('Failed');
    const posts = await res.json();

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
        <td><button class="delete-btn" onclick="deletePost(${post.id})">Delete</button></td>
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
  document.getElementById('logout-btn').addEventListener('click', handleLogout);
  initUmamiTracking();
  checkAuth();
});
