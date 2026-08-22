// RedChef — Auth-Aware Feed Client

const API = {
  me: '/api/auth/me',
  login: '/api/auth/login',
  register: '/api/auth/register',
  logout: '/api/auth/logout',
  forgot: '/api/auth/forgot',
  reset: '/api/auth/reset',
  posts: '/api/posts',
  unlock: '/api/pay/unlock',
  payItem: '/api/pay/item',
  analytics: '/api/settings/analytics',
  favourites: '/api/favourites',
};

let currentUser = null;
let posts = [];
let currentTab = 'posts';

// ── DOM refs ──
const $ = id => document.getElementById(id);

// ── API token (second credential for mutations) ──
const API_TOKEN_KEY = 'redchef_api_token';

function getStoredApiToken() {
  return localStorage.getItem(API_TOKEN_KEY) || '';
}

function authedFetch(url, opts = {}) {
  const token = getStoredApiToken();
  const headers = Object.assign({}, opts.headers || {});
  if (token) headers['Authorization'] = 'Bearer ' + token;
  return fetch(url, Object.assign({}, opts, { headers })).catch(err => {
    throw err;
  }).then(async res => {
    if (res.status === 401) {
      try {
        const data = await res.clone().json();
        if (data.error && data.error.includes('API token')) {
          localStorage.removeItem(API_TOKEN_KEY);
          showToast('🔑 This action needs a valid API token — create one under "API Tokens".', true);
        }
      } catch (_) {}
    }
    return res;
  });
}

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

  $('tab-favourites').style.display = 'inline-flex';

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
  $('tab-favourites').style.display = 'none';
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

// ── Forgot / Reset Password ──

function openForgotModal() {
  closeLoginModal();
  const form = $('forgot-form');
  // Restore the form if it was replaced by the success state
  if (form.querySelector('.auth-success')) {
    form.innerHTML = `
      <div class="form-group">
        <label>Email</label>
        <input type="email" id="forgot-email" placeholder="your@email.com" autocomplete="email" required>
      </div>
      <button type="submit" class="btn-primary">Send Reset Link</button>
      <div class="auth-error" id="forgot-error"></div>
    `;
    $('forgot-form').addEventListener('submit', handleForgot);
  }
  $('forgot-modal').style.display = 'flex';
  $('forgot-error').textContent = '';
  $('forgot-email').value = '';
}

function closeForgotModal() {
  $('forgot-modal').style.display = 'none';
}

async function handleForgot(e) {
  e.preventDefault();
  const email = $('forgot-email').value.trim();
  const err = $('forgot-error');

  err.textContent = '';
  if (!email || !email.includes('@')) { err.textContent = 'Enter a valid email'; return; }

  const btn = e.target.querySelector('button[type="submit"]');
  if (btn) { btn.disabled = true; btn.textContent = 'Sending...'; }

  try {
    const res = await fetch(API.forgot, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    });
    const data = await res.json();
    if (!res.ok) { err.textContent = data.error || 'Request failed'; return; }

    // Always show the same message so accounts can't be enumerated
    $('forgot-form').innerHTML = `
      <div class="auth-success">
        <div style="font-size:2rem;margin-bottom:8px;">📬</div>
        <p>If an account exists for <strong>${escapeHtml(email)}</strong>, a reset link is on its way. Check your inbox (and spam folder).</p>
      </div>
    `;
  } catch (_) {
    err.textContent = 'Network error — is the server running?';
  } finally {
    if (btn) { btn.disabled = false; btn.textContent = 'Send Reset Link'; }
  }
}

function openResetModal() {
  $('reset-modal').style.display = 'flex';
  $('reset-error').textContent = '';
}

function closeResetModal() {
  $('reset-modal').style.display = 'none';
}

async function handleReset(e) {
  e.preventDefault();
  const token = new URLSearchParams(window.location.search).get('token') || '';
  const password = $('reset-password').value;
  const confirm = $('reset-confirm').value;
  const err = $('reset-error');

  err.textContent = '';
  if (!token) { err.textContent = 'Missing reset token. Request a new link.'; return; }
  if (password.length < 6) { err.textContent = 'Password must be at least 6 characters'; return; }
  if (password !== confirm) { err.textContent = 'Passwords do not match'; return; }

  try {
    const res = await fetch(API.reset, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token, password, confirm_password: confirm }),
    });
    const data = await res.json();
    if (!res.ok) { err.textContent = data.error || 'Reset failed'; return; }

    closeResetModal();
    // Remove the token from the URL so it doesn't linger in history
    history.replaceState({}, '', window.location.pathname);
    showToast('🔑 Password updated! Sign in with your new password.');
    openLoginModal();
  } catch (_) {
    err.textContent = 'Network error — is the server running?';
  }
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

// ── Paywall (mock iDEAL — always succeeds) ──

let purchaseMode = 'subscription'; // 'subscription' | 'item'
let purchasePostId = null;

function handleUnlock() {
  if (!currentUser) {
    openLoginModal();
    return;
  }
  purchaseMode = 'subscription';
  purchasePostId = null;
  openIdealModal();
}

function buyItem(postId, mediaType) {
  if (!currentUser) {
    openLoginModal();
    return;
  }
  purchaseMode = 'item';
  purchasePostId = postId;
  const label = mediaType === 'video' ? '€0,20' : '€0,05';
  const itemLabel = mediaType === 'video' ? 'video' : 'foto';
  $('modal-icon').textContent = '🔓';
  $('modal-title').textContent = 'Ontgrendel deze ' + itemLabel;
  $('modal-subtitle').innerHTML = `Betaal <strong>${label}</strong> — eenmalig, direct toegang`;
  $('modal-btn-text').textContent = `Betaal ${label} met iDEAL`;
  openIdealModal();
}

function openIdealModal() {
  if (purchaseMode === 'subscription') {
    $('modal-icon').textContent = '👑';
    $('modal-title').textContent = 'Word Royal Member';
    $('modal-subtitle').innerHTML = 'Onbeperkt toegang voor <strong>€4,99/maand</strong>';
    $('modal-btn-text').textContent = 'Betaal €4,99 met iDEAL';
  }
  $('ideal-modal').style.display = 'flex';
  $('bank-select').value = '';
}

function closeIdealModal() {
  $('ideal-modal').style.display = 'none';
  purchaseMode = 'subscription';
  purchasePostId = null;
}

async function handleIdealSubmit(e) {
  e.preventDefault();

  const bank = $('bank-select').value;
  if (!bank) {
    showToast('❌ Selecteer een bank om te betalen.', true);
    return;
  }

  const isItem = purchaseMode === 'item' && purchasePostId;
  const payBtn = $('ideal-pay-btn');
  payBtn.disabled = true;
  $('modal-btn-text').textContent = `Verwerken via ${bank}...`;

  try {
    const res = await authedFetch(isItem ? API.payItem : API.unlock, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(isItem ? { bank, post_id: purchasePostId } : { bank }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed');

    closeIdealModal();
    showToast((isItem ? '🏦 ' : '👑 ') + data.message);
    if (!isItem) {
      currentUser.paid = true;
      showLoggedIn();
    }
    loadContent();
  } catch (err) {
    showToast('❌ Betaling mislukt — probeer het opnieuw.', true);
  } finally {
    payBtn.disabled = false;
    $('modal-btn-text').textContent = 'Betaal €4,99 met iDEAL';
  }
}

// ── Feed ──

async function loadContent() {
  const feed = $('feed');

  // Shared single-post view (/posts/{id})
  const sharedMatch = window.location.pathname.match(/^\/posts\/(\d+)$/);
  if (sharedMatch) {
    await loadSinglePost(parseInt(sharedMatch[1], 10));
    return;
  }

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
    let url;
    if (currentTab === 'favourites') {
      url = API.favourites;
    } else {
      url = API.posts + (params.toString() ? '?' + params.toString() : '');
    }
    const res = await fetch(url);
    if (!res.ok) throw new Error('Failed to load');
    posts = await res.json();

    $('stat-posts').textContent = posts.length;

    if (currentTab === 'media') {
      renderMediaView();
    } else if (currentTab === 'favourites') {
      renderPostsView();
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
    const emptyMsg = currentTab === 'favourites'
      ? 'No favourites yet. ❤️ posts to save them here!'
      : 'The Chef is still cooking... Check back soon for some sizzling content!';
    feed.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">${currentTab === 'favourites' ? '❤️' : '🍳'}</div>
        <p>${emptyMsg}</p>
      </div>
    `;
    return;
  }

  const isPaid = currentUser && (currentUser.paid || currentUser.role === 'admin');

  feed.innerHTML = posts.map((post, idx) => {
    const unlocked = post.unlocked !== undefined ? post.unlocked : (!post.locked || isPaid);
    const mediaUrl = `/uploads/${post.filename}`;
    const thumbnailUrl = post.thumbnail && post.thumbnail !== post.filename
      ? `/uploads/${post.thumbnail}`
      : mediaUrl;
    const date = new Date(post.created_at).toLocaleDateString('en-US', {
      year: 'numeric', month: 'short', day: 'numeric'
    });
    const postUrl = window.location.origin + '/posts/' + post.id;

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
      const itemPrice = post.media_type === 'video' ? '€0,20' : '€0,05';
      const itemLabel = post.media_type === 'video' ? 'video' : 'foto';
      mediaHtml = `
        <div class="post-media-locked">
          <img class="post-media-blur" src="${thumbnailUrl}" alt="" loading="lazy"
               onerror="this.style.display='none';this.parentElement.style.background='#222'">
          <div class="post-locked-overlay">
            <div class="post-locked-icon">🔒</div>
            <div class="post-locked-text">Alleen voor leden</div>
            <button class="post-locked-btn" onclick="buyItem(${post.id}, '${post.media_type}')">Koop deze ${itemLabel} — ${itemPrice}</button>
            <button class="post-locked-sub" onclick="handleUnlock()">👑 Word lid — €4,99/maand</button>
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

    // Linked posts series navigation
    let linkedHtml = '';
    if (post.linked_posts && post.linked_posts.length > 0) {
      linkedHtml = `
        <div class="linked-posts">
          <span class="linked-posts-label">📎 Part of this series:</span>
          ${post.linked_posts.map(lp => `
            <a href="/posts/${lp.linked_post_id}" class="linked-post-link" data-id="${lp.linked_post_id}">${escapeHtml(lp.linked_title)}</a>
          `).join('')}
        </div>
      `;
    }

    // Heart icon state
    const heartIcon = post.favourited ? '❤️' : '🤍';
    const heartClass = post.favourited ? 'action-btn active' : 'action-btn';

    return `
      <div class="post-card" data-post-id="${post.id}">
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
        ${linkedHtml}
        <div class="post-actions">
          <button class="action-btn" onclick="toggleComments(${post.id}, this)">
            <span class="action-icon">💬</span>
            <span class="action-count" data-comment-count="${post.id}">0</span>
          </button>
          <button class="${heartClass}" onclick="handleFavourite(${post.id}, this)">
            <span class="action-icon">${heartIcon}</span>
            <span class="action-count" data-fav-count="${post.id}">${post.favourite_count || 0}</span>
          </button>
          <button class="action-btn" onclick="openTipModal(${post.id})" ${!currentUser ? 'disabled' : ''}>
            <span class="action-icon">💸</span>
            <span class="action-count" data-tip-count="${post.id}">${post.tip_count || 0}</span>
          </button>
          <button class="action-btn" onclick="handleWhatsAppShare('${escapeHtml(post.title)}', ${post.id})">
            <span class="action-icon">📱</span>
          </button>
        </div>
        <div class="comments-section" id="comments-${post.id}" style="display:none;">
          <div class="comments-loading" id="comments-loading-${post.id}">Loading comments...</div>
          <div class="comments-list" id="comments-list-${post.id}"></div>
          ${currentUser ? `
            <div class="comment-form">
              <textarea class="comment-input" id="comment-input-${post.id}" placeholder="Write a comment..." rows="2"></textarea>
              <button class="filter-btn" onclick="submitComment(${post.id})">Post</button>
            </div>
          ` : `
            <div class="comment-login-msg"><a onclick="openLoginModal()">Sign in</a> to leave a comment</div>
          `}
        </div>
      </div>
    `;
  }).join('');

  // Load comment counts for all posts
  posts.forEach(post => loadCommentCount(post.id));
}

// ── Shared single-post view ──

async function loadSinglePost(postId) {
  const feed = $('feed');
  try {
    const res = await fetch(API.posts + '/' + postId);
    if (!res.ok) throw new Error('Not found');
    const post = await res.json();

    posts = [post];
    $('stat-posts').textContent = '1';

    // Hide feed controls in single-post mode
    const filterBar = document.querySelector('.filter-bar');
    const tabsBar = document.querySelector('.tabs-bar');
    if (filterBar) filterBar.style.display = 'none';
    if (tabsBar) tabsBar.style.display = 'none';

    renderPostsView();
    feed.insertAdjacentHTML('afterbegin',
      '<div style="margin-bottom:1rem;"><a class="filter-btn" style="text-decoration:none;" href="/">← Alle posts bekijken</a></div>');
  } catch (_) {
    feed.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">🔍</div>
        <p>Deze post bestaat niet (meer).</p>
        <p style="margin-top:1rem;"><a class="filter-btn" style="text-decoration:none;" href="/">← Terug naar de feed</a></p>
      </div>
    `;
  }
}

// ── Favourites ──

async function handleFavourite(postId, btn) {
  if (!currentUser) { openLoginModal(); return; }
  try {
    const res = await authedFetch('/api/posts/' + postId + '/favourite', { method: 'POST' });
    if (!res.ok) throw new Error('Failed');
    const data = await res.json();
    // Toggle heart icon
    const icon = btn.querySelector('.action-icon');
    icon.textContent = data.favourited ? '❤️' : '🤍';
    btn.classList.toggle('active', data.favourited);
    // Update count
    const countSpan = btn.querySelector('.action-count');
    countSpan.textContent = data.favourite_count;
  } catch (_) {
    showToast('❌ Failed to toggle favourite');
  }
}

// ── Tips ──

function openTipModal(postId) {
  if (!currentUser) { openLoginModal(); return; }

  // Remove existing tip modal if any
  const existing = document.getElementById('tip-modal');
  if (existing) existing.remove();

  const overlay = document.createElement('div');
  overlay.className = 'auth-modal-overlay';
  overlay.id = 'tip-modal';
  overlay.style.display = 'flex';
  overlay.innerHTML = `
    <div class="auth-modal" style="max-width:380px;">
      <button class="modal-close" onclick="this.closest('.auth-modal-overlay').remove()">✕</button>
      <h2>💸 Geef een fooi</h2>
      <p class="auth-subtitle">Waardeer de Chef met een kleine bijdrage</p>
      <div class="tip-presets" style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:1rem;justify-content:center;">
        <button class="tip-preset-btn" data-amount="100" style="padding:10px 18px;border:2px solid #444;border-radius:8px;background:#222;color:var(--white);font-size:1rem;cursor:pointer;transition:all 0.2s;">€1,00</button>
        <button class="tip-preset-btn" data-amount="250" style="padding:10px 18px;border:2px solid #444;border-radius:8px;background:#222;color:var(--white);font-size:1rem;cursor:pointer;transition:all 0.2s;">€2,50</button>
        <button class="tip-preset-btn" data-amount="500" style="padding:10px 18px;border:2px solid #444;border-radius:8px;background:#222;color:var(--white);font-size:1rem;cursor:pointer;transition:all 0.2s;">€5,00</button>
        <button class="tip-preset-btn" data-amount="1000" style="padding:10px 18px;border:2px solid #444;border-radius:8px;background:#222;color:var(--white);font-size:1rem;cursor:pointer;transition:all 0.2s;">€10,00</button>
      </div>
      <div class="form-group">
        <label>Of voer bedrag in (in centen)</label>
        <input type="number" id="tip-amount-input" min="1" step="1" value="100" placeholder="b.v. 100 = €1,00"
               style="width:100%;padding:12px 14px;border-radius:8px;border:1px solid #444;background:#222;color:var(--white);font-size:1.2rem;font-family:inherit;text-align:center;">
      </div>
      <button class="btn-primary" id="tip-send-btn" data-post-id="${postId}" style="margin-top:0;">💸 Verstuur fooi</button>
    </div>
  `;
  document.body.appendChild(overlay);

  // Click outside to close
  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) overlay.remove();
  });

  // Preset button selection
  overlay.querySelectorAll('.tip-preset-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      overlay.querySelectorAll('.tip-preset-btn').forEach(b => {
        b.style.borderColor = '#444';
        b.style.background = '#222';
      });
      btn.style.borderColor = 'var(--gold)';
      btn.style.background = 'rgba(245,197,24,0.15)';
      document.getElementById('tip-amount-input').value = btn.dataset.amount;
    });
  });

  // Submit
  document.getElementById('tip-send-btn').addEventListener('click', async () => {
    const amountCents = parseInt(document.getElementById('tip-amount-input').value);
    if (!amountCents || amountCents < 1) {
      showToast('❌ Voer een geldig bedrag in (minimaal 1 cent)');
      return;
    }
    await sendTip(postId, amountCents);
    overlay.remove();
  });
}

async function sendTip(postId, amountCents) {
  try {
    const res = await authedFetch('/api/posts/' + postId + '/tip', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ amount_cents: amountCents }),
    });
    if (!res.ok) {
      const data = await res.json();
      showToast('❌ ' + (data.error || 'Failed'));
      return;
    }
    const data = await res.json();
    // Update tip count in the feed by finding the button in the post card
    const postCard = document.querySelector(`.post-card[data-post-id="${postId}"]`);
    if (postCard) {
      const actionBtns = postCard.querySelectorAll('.action-btn');
      for (const btn of actionBtns) {
        const icon = btn.querySelector('.action-icon');
        if (icon && icon.textContent === '💸') {
          const countSpan = btn.querySelector('.action-count');
          if (countSpan) countSpan.textContent = data.tip_count;
        }
      }
    }
    showToast('🎉 Bedankt voor de fooi van ' + data.formatted + '!');
  } catch (_) {
    showToast('❌ Failed to send tip');
  }
}

// ── WhatsApp Share ──

function handleWhatsAppShare(title, postId) {
  const url = window.location.origin + '/posts/' + postId;
  const text = encodeURIComponent(title + '\n' + url);
  window.open('https://wa.me/?text=' + text, '_blank');
}

// ── Comments ──

async function loadCommentCount(postId) {
  try {
    const res = await fetch('/api/posts/' + postId + '/comments');
    if (!res.ok) return;
    const comments = await res.json();
    const countSpan = document.querySelector(`[data-comment-count="${postId}"]`);
    if (countSpan) countSpan.textContent = comments.length;
  } catch (_) {}
}

async function toggleComments(postId, btn) {
  const section = document.getElementById('comments-' + postId);
  if (!section) return;
  const isOpen = section.style.display !== 'none';
  section.style.display = isOpen ? 'none' : 'block';
  if (!isOpen) {
    await loadComments(postId);
  }
}

async function loadComments(postId) {
  const list = document.getElementById('comments-list-' + postId);
  const loading = document.getElementById('comments-loading-' + postId);
  if (!list) return;
  try {
    if (loading) loading.style.display = 'block';
    const res = await fetch('/api/posts/' + postId + '/comments');
    if (!res.ok) throw new Error('Failed');
    const comments = await res.json();
    if (loading) loading.style.display = 'none';
    renderComments(postId, comments, list);
  } catch (_) {
    if (loading) loading.style.display = 'none';
    list.innerHTML = '<div class="comment-error">Failed to load comments.</div>';
  }
}

function renderComments(postId, comments, container) {
  if (comments.length === 0) {
    container.innerHTML = '<div class="comment-empty">No comments yet. Be the first!</div>';
    return;
  }

  // Build tree from flat list with parent_id
  const tree = {};
  const roots = [];
  comments.forEach(c => {
    tree[c.id] = { ...c, replies: [] };
  });
  comments.forEach(c => {
    if (c.parent_id && tree[c.parent_id]) {
      tree[c.parent_id].replies.push(tree[c.id]);
    } else {
      roots.push(tree[c.id]);
    }
  });

  function renderCommentNode(node, depth) {
    const indent = depth > 0 ? 'margin-left:' + (depth * 20) + 'px;' : '';
    return `
      <div class="comment" style="${indent}">
        <div class="comment-avatar">👤</div>
        <div class="comment-body">
          <div class="comment-author">${escapeHtml(node.username || 'User')}</div>
          <div class="comment-text">${escapeHtml(node.body)}</div>
          <div class="comment-meta">
            <span class="comment-date">${new Date(node.created_at).toLocaleDateString()}</span>
            ${currentUser ? `<a class="comment-reply-link" onclick="openReplyForm(${postId}, ${node.id})">Reply</a>` : ''}
          </div>
          ${node.replies.length > 0 ? node.replies.map(r => renderCommentNode(r, depth + 1)).join('') : ''}
        </div>
      </div>
    `;
  }

  container.innerHTML = roots.map(r => renderCommentNode(r, 0)).join('');
}

function openReplyForm(postId, parentId) {
  const list = document.getElementById('comments-list-' + postId);
  // Remove any existing reply forms
  list.querySelectorAll('.comment-reply-form').forEach(el => el.remove());

  const form = document.createElement('div');
  form.className = 'comment-reply-form';
  form.style.marginLeft = '40px';
  form.innerHTML = `
    <textarea class="comment-input" id="reply-input-${postId}-${parentId}" placeholder="Write a reply..." rows="2"></textarea>
    <div style="display:flex;gap:8px;margin-top:8px;">
      <button class="filter-btn" onclick="submitReply(${postId}, ${parentId})">Reply</button>
      <button class="filter-btn" style="background:#555;" onclick="this.parentElement.parentElement.remove()">Cancel</button>
    </div>
  `;
  list.appendChild(form);
  document.getElementById('reply-input-' + postId + '-' + parentId).focus();
}

async function submitComment(postId) {
  const input = document.getElementById('comment-input-' + postId);
  const body = input.value.trim();
  if (!body) return;
  await submitCommentBody(postId, body, null);
  input.value = '';
}

async function submitReply(postId, parentId) {
  const input = document.getElementById('reply-input-' + postId + '-' + parentId);
  const body = input.value.trim();
  if (!body) return;
  await submitCommentBody(postId, body, parentId);
  // Remove reply form
  const form = input.closest('.comment-reply-form');
  if (form) form.remove();
  // Reload comments
  await loadComments(postId);
}

async function submitCommentBody(postId, body, parentId) {
  try {
    const payload = { body };
    if (parentId) payload.parent_id = parentId;
    const res = await authedFetch('/api/posts/' + postId + '/comments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const data = await res.json();
      showToast('❌ ' + (data.error || 'Failed to post comment'));
      return;
    }
    showToast('💬 Comment posted!');
    await loadComments(postId);
    await loadCommentCount(postId);
  } catch (_) {
    showToast('❌ Failed to post comment');
  }
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
    const unlocked = post.unlocked !== undefined ? post.unlocked : (!post.locked || isPaid);

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

// ── API Tokens UI ──

function openTokensModal() {
  $('tokens-modal').style.display = 'flex';
  $('token-secret-box').style.display = 'none';
  loadTokens();
}

function closeTokensModal() {
  $('tokens-modal').style.display = 'none';
}

async function loadTokens() {
  const listEl = $('token-list');
  try {
    const res = await fetch('/api/tokens');
    if (!res.ok) { listEl.innerHTML = '<p class="auth-subtitle">Could not load tokens.</p>'; return; }
    const tokens = await res.json();
    if (!tokens.length) {
      listEl.innerHTML = '<p class="auth-subtitle">No tokens yet.</p>';
      return;
    }
    listEl.innerHTML = tokens.map(t => {
      const state = t.revoked_at ? '❌ revoked' : (t.expires_at && new Date(t.expires_at) < new Date() ? '⏰ expired' : '✅ active');
      const lastUsed = t.last_used_at ? new Date(t.last_used_at).toLocaleString() : 'never';
      return `<div class="form-group" style="border:1px solid #ddd;border-radius:8px;padding:10px;">
        <strong>${escapeHtml(t.name)}</strong> <small>${state}</small><br>
        <small>created ${new Date(t.created_at).toLocaleString()} · last used ${lastUsed}</small><br>
        ${!t.revoked_at ? `<button type="button" class="auth-switch-link" data-token-action="rotate" data-id="${t.id}">Rotate</button>
        <button type="button" class="auth-switch-link" data-token-action="revoke" data-id="${t.id}">Revoke</button>` : ''}
      </div>`;
    }).join('');
    listEl.querySelectorAll('[data-token-action]').forEach(btn => {
      btn.addEventListener('click', () => handleTokenAction(btn.dataset.tokenAction, btn.dataset.id));
    });
  } catch (_) {
    listEl.innerHTML = '<p class="auth-subtitle">Could not load tokens.</p>';
  }
}

function revealTokenSecret(secret) {
  localStorage.setItem(API_TOKEN_KEY, secret);
  $('token-secret-box').style.display = 'block';
  $('token-secret-value').textContent = secret;
}

async function handleTokenCreate(e) {
  e.preventDefault();
  const name = $('token-name').value.trim();
  if (!name) return;
  try {
    const res = await fetch('/api/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
    const data = await res.json();
    if (!res.ok) { showToast('❌ ' + (data.error || 'Failed to create token'), true); return; }
    $('token-name').value = '';
    revealTokenSecret(data.token);
    loadTokens();
  } catch (_) {
    showToast('❌ Failed to create token', true);
  }
}

async function handleTokenAction(action, id) {
  const url = action === 'rotate' ? `/api/tokens/${id}/rotate` : `/api/tokens/${id}`;
  try {
    const res = await fetch(url, { method: action === 'rotate' ? 'POST' : 'DELETE' });
    const data = await res.json();
    if (!res.ok) { showToast('❌ ' + (data.error || 'Failed'), true); return; }
    if (action === 'rotate') revealTokenSecret(data.token);
    else showToast('🔑 Token revoked');
    loadTokens();
  } catch (_) {
    showToast('❌ Failed', true);
  }
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

  // Forgot / reset password modals
  $('switch-to-forgot').addEventListener('click', openForgotModal);
  $('forgot-modal-close').addEventListener('click', closeForgotModal);
  $('forgot-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeForgotModal();
  });
  $('forgot-form').addEventListener('submit', handleForgot);
  $('switch-forgot-to-login').addEventListener('click', () => {
    closeForgotModal();
    openLoginModal();
  });
  $('reset-modal-close').addEventListener('click', closeResetModal);
  $('reset-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeResetModal();
  });
  $('reset-form').addEventListener('submit', handleReset);

  // Open the reset modal when arriving with a reset token
  if (new URLSearchParams(window.location.search).get('token')) {
    openResetModal();
  }

  // Paywall
  $('btn-unlock').addEventListener('click', handleUnlock);

  // API tokens
  $('btn-tokens').addEventListener('click', openTokensModal);
  $('tokens-modal-close').addEventListener('click', closeTokensModal);
  $('tokens-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeTokensModal();
  });
  $('token-create-form').addEventListener('submit', handleTokenCreate);
  $('token-copy-btn').addEventListener('click', () => {
    navigator.clipboard.writeText($('token-secret-value').textContent)
      .then(() => showToast('🔑 Token copied'));
  });

  // iDEAL modal
  $('ideal-modal-close').addEventListener('click', closeIdealModal);
  $('ideal-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeIdealModal();
  });
  $('ideal-form').addEventListener('submit', handleIdealSubmit);

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
