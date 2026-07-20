// RedChef — Auth-Aware Feed Client

const API = {
  me: '/api/auth/me',
  login: '/api/auth/login',
  register: '/api/auth/register',
  logout: '/api/auth/logout',
  posts: '/api/posts',
  unlock: '/api/pay/unlock',
  analytics: '/api/settings/analytics',
};

let currentUser = null;
let posts = [];
let currentTab = 'posts';

// ── DOM refs ──
const $ = id => document.getElementById(id);

// ── Escape ──
function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// ── Auth ──

async function checkAuth() {
  try {
    const res = await fetch(API.me);
    const data = await res.json();
    if (data.authenticated) {
      currentUser = data;
      showLoggedIn();
    } else {
      showLoggedOut();
    }
  } catch (_) {
    showLoggedOut();
  }
}

function showLoggedIn() {
  $('btn-login').style.display = 'none';
  $('btn-register').style.display = 'none';
  $('nav-user').style.display = 'flex';

  $('nav-email').textContent = currentUser.email;

  if (currentUser.paid || currentUser.role === 'admin') {
    $('nav-paid-badge').style.display = 'inline-flex';
    $('paywall-banner').style.display = 'none';
  } else {
    $('nav-paid-badge').style.display = 'none';
    $('paywall-banner').style.display = 'block';
  }

  if (currentUser.role === 'admin') {
    $('nav-admin-link').style.display = 'inline-flex';
  } else {
    $('nav-admin-link').style.display = 'none';
  }
}

function showLoggedOut() {
  $('btn-login').style.display = 'inline-flex';
  $('btn-register').style.display = 'inline-flex';
  $('nav-user').style.display = 'none';
  $('paywall-banner').style.display = 'none';
  currentUser = null;
}

async function handleLogin(e) {
  e.preventDefault();
  const email = $('login-email').value.trim();
  const password = $('login-password').value;
  const err = $('login-error');

  err.textContent = '';
  if (!email || !password) { err.textContent = 'All fields required'; return; }

  try {
    const res = await fetch(API.login, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });
    const data = await res.json();
    if (!res.ok) { err.textContent = data.error || 'Login failed'; return; }

    closeLoginModal();
    $('login-email').value = '';
    $('login-password').value = '';
    await checkAuth();
    loadContent();
  } catch (_) {
    err.textContent = 'Network error — is the server running?';
  }
}

async function handleRegister(e) {
  e.preventDefault();
  const email = $('register-email').value.trim();
  const password = $('register-password').value;
  const confirm = $('register-confirm').value;
  const err = $('register-error');

  err.textContent = '';
  if (!email) { err.textContent = 'Email is required'; return; }
  if (!email.includes('@')) { err.textContent = 'Enter a valid email'; return; }
  if (password.length < 6) { err.textContent = 'Password must be at least 6 characters'; return; }
  if (password !== confirm) { err.textContent = 'Passwords do not match'; return; }

  try {
    const res = await fetch(API.register, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, confirm_password: confirm }),
    });
    const data = await res.json();
    if (!res.ok) { err.textContent = data.error || 'Registration failed'; return; }

    closeRegisterModal();
    $('register-email').value = '';
    $('register-password').value = '';
    $('register-confirm').value = '';
    await checkAuth();
    loadContent();
    showToast('✅ Account created! Welcome to the kitchen.');
  } catch (_) {
    err.textContent = 'Network error';
  }
}

async function handleLogout() {
  try {
    await fetch(API.logout, { method: 'POST' });
  } catch (_) {}
  currentUser = null;
  showLoggedOut();
  loadContent();
}

// ── Modals ──

function openLoginModal() {
  $('login-modal').style.display = 'flex';
  $('login-error').textContent = '';
}

function closeLoginModal() {
  $('login-modal').style.display = 'none';
}

function openRegisterModal() {
  $('register-modal').style.display = 'flex';
  $('register-error').textContent = '';
}

function closeRegisterModal() {
  $('register-modal').style.display = 'none';
}

// ── Paywall ──

async function handleUnlock() {
  if (!currentUser) {
    openLoginModal();
    return;
  }

  try {
    $('btn-unlock').disabled = true;
    $('btn-unlock').textContent = 'Processing...';
    const res = await fetch(API.unlock, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed');

    showToast('👑 ' + data.message);
    currentUser.paid = true;
    showLoggedIn();
    loadContent();
  } catch (err) {
    showToast('❌ Failed to process. Try again.');
  } finally {
    $('btn-unlock').disabled = false;
    $('btn-unlock').textContent = 'Unlock Everything — $5.00/mo';
  }
}

// ── Feed ──

async function loadContent() {
  const feed = $('feed');
  const params = new URLSearchParams();

  const sort = $('filter-sort').value;
  const type = $('filter-type').value;
  const dateFrom = $('filter-date-from').value;
  const dateTo = $('filter-date-to').value;

  if (sort && sort !== 'newest') params.set('sort', sort);
  if (type) params.set('type', type);
  if (dateFrom) params.set('date_from', dateFrom);
  if (dateTo) params.set('date_to', dateTo);

  try {
    const url = API.posts + (params.toString() ? '?' + params.toString() : '');
    const res = await fetch(url);
    if (!res.ok) throw new Error('Failed to load');
    posts = await res.json();

    $('stat-posts').textContent = posts.length;

    if (currentTab === 'media') {
      renderMediaView();
    } else {
      renderPostsView();
    }
  } catch (err) {
    feed.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">😰</div>
        <p>Couldn't reach the Chef's kitchen. Try again later!</p>
      </div>
    `;
  }
}

function renderPostsView() {
  const feed = $('feed');

  if (posts.length === 0) {
    feed.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">🍳</div>
        <p>The Chef is still cooking... Check back soon for some sizzling content!</p>
      </div>
    `;
    return;
  }

  const isPaid = currentUser && (currentUser.paid || currentUser.role === 'admin');

  feed.innerHTML = posts.map(post => {
    const unlocked = !post.locked || isPaid;
    const mediaUrl = `/uploads/${post.filename}`;
    const thumbnailUrl = post.thumbnail && post.thumbnail !== post.filename
      ? `/uploads/${post.thumbnail}`
      : mediaUrl;
    const date = new Date(post.created_at).toLocaleDateString('en-US', {
      year: 'numeric', month: 'short', day: 'numeric'
    });

    let mediaHtml;
    if (post.processing) {
      mediaHtml = `
        <div class="post-media" style="background:#111;display:flex;align-items:center;justify-content:center;min-height:200px;">
          <div style="text-align:center;padding:2rem;">
            <div style="font-size:2rem;margin-bottom:8px;">⏳</div>
            <div style="color:#888;font-size:0.85rem;">Processing media...</div>
          </div>
        </div>
      `;
    } else if (post.locked && !unlocked) {
      mediaHtml = `
        <div class="post-media-locked">
          <img class="post-media-blur" src="${thumbnailUrl}" alt="" loading="lazy"
               onerror="this.style.display='none';this.parentElement.style.background='#222'">
          <div class="post-locked-overlay">
            <div class="post-locked-icon">🔒</div>
            <div class="post-locked-text">Members Only</div>
            <p style="color:rgba(255,255,255,0.7);font-size:0.85rem;margin:4px 0;">Unlock all content with membership</p>
          </div>
        </div>
      `;
    } else if (post.media_type === 'video') {
      mediaHtml = `
        <div class="post-media">
          <video controls preload="metadata">
            <source src="${mediaUrl}" type="video/mp4">
          </video>
        </div>
      `;
    } else {
      mediaHtml = `
        <div class="post-media">
          <img src="${mediaUrl}" alt="${escapeHtml(post.title)}" loading="lazy"
               onerror="this.style.display='none';this.parentElement.style.background='#222'">
        </div>
      `;
    }

    return `
      <div class="post-card">
        <div class="post-header">
          <div class="post-header-avatar">🍳</div>
          <div class="post-header-info">
            <div class="post-header-name">
              Red Copper Chef
              <span class="verified-badge" title="Verified Chef">✓</span>
            </div>
            <div class="post-header-date">${date}</div>
          </div>
        </div>
        <div class="post-caption">
          <div class="post-title">${escapeHtml(post.title)}</div>
          ${post.description ? `<div class="post-desc">${escapeHtml(post.description)}</div>` : ''}
        </div>
        ${mediaHtml}
        <div class="post-actions">
          <button class="action-btn" disabled>
            <span class="action-icon">💬</span>
            <span>0</span>
          </button>
          <button class="action-btn" disabled>
            <span class="action-icon">💸</span>
            <span>Tip</span>
          </button>
        </div>
      </div>
    `;
  }).join('');
}

function renderMediaView() {
  const feed = $('feed');
  const isPaid = currentUser && (currentUser.paid || currentUser.role === 'admin');

  if (posts.length === 0) {
    feed.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">📸</div>
        <p>No media yet. Upload something to get started!</p>
      </div>
    `;
    return;
  }

  feed.innerHTML = posts.map(post => {
    const mediaUrl = `/uploads/${post.filename}`;
    const unlocked = !post.locked || isPaid;

    let mediaEl;
    if (post.processing) {
      mediaEl = `<div style="display:flex;align-items:center;justify-content:center;height:100%;background:#111;color:#888;font-size:1.5rem;">⏳</div>`;
    } else if (post.media_type === 'video') {
      mediaEl = `<video src="${mediaUrl}" preload="metadata"></video>`;
    } else {
      mediaEl = `<img src="${mediaUrl}" alt="${escapeHtml(post.title)}" loading="lazy">`;
    }

    if (!unlocked && !post.processing) {
      mediaEl = `<div class="media-grid-locked"><div class="media-grid-blur">${mediaEl}</div><div class="media-grid-overlay">🔒</div></div>`;
    }

    return `
      <div class="media-grid-item">
        ${mediaEl}
        <div class="media-grid-info">
          <span class="media-grid-title">${escapeHtml(post.title)}</span>
          <span class="media-grid-date">${new Date(post.created_at).toLocaleDateString()}</span>
        </div>
      </div>
    `;
  }).join('');
}

// ── Tab Switching ──

function switchTab(tab) {
  currentTab = tab;
  document.querySelectorAll('.tab').forEach(t => {
    t.classList.toggle('tab-active', t.dataset.tab === tab);
  });
  const feed = $('feed');
  feed.className = 'feed' + (tab === 'media' ? ' feed-media' : '');

  if (tab === 'media') renderMediaView();
  else renderPostsView();
}

// ── Toast ──

function showToast(message, isError) {
  const container = $('toast-container');
  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.innerHTML = `
    <div class="toast-icon">${isError ? '❌' : '✅'}</div>
    <div class="toast-text">${message}</div>
  `;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.transition = 'opacity 0.5s';
    toast.style.opacity = '0';
    setTimeout(() => toast.remove(), 500);
  }, 4000);
}

// ── Umami Tracking ──

async function initUmamiTracking() {
  try {
    const res = await fetch(API.analytics);
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

// ── Init ──

document.addEventListener('DOMContentLoaded', () => {
  initUmamiTracking();

  // Auth
  $('btn-login').addEventListener('click', openLoginModal);
  $('btn-register').addEventListener('click', openRegisterModal);
  $('btn-logout').addEventListener('click', handleLogout);

  // Login modal
  $('login-modal-close').addEventListener('click', closeLoginModal);
  $('login-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeLoginModal();
  });
  $('login-form').addEventListener('submit', handleLogin);

  // Register modal
  $('register-modal-close').addEventListener('click', closeRegisterModal);
  $('register-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeRegisterModal();
  });
  $('register-form').addEventListener('submit', handleRegister);

  // Switch between login/register
  $('switch-to-register').addEventListener('click', () => {
    closeLoginModal();
    openRegisterModal();
  });
  $('switch-to-login').addEventListener('click', () => {
    closeRegisterModal();
    openLoginModal();
  });

  // Paywall
  $('btn-unlock').addEventListener('click', handleUnlock);

  // Filter
  $('btn-filter-apply').addEventListener('click', loadContent);
  // Auto-filter on Enter in date fields
  $('filter-date-from').addEventListener('keydown', e => { if (e.key === 'Enter') loadContent(); });
  $('filter-date-to').addEventListener('keydown', e => { if (e.key === 'Enter') loadContent(); });

  // Tab switching
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => switchTab(tab.dataset.tab));
  });

  // Load
  checkAuth().then(loadContent);
});
