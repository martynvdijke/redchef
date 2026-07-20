// RedChef — Royal Fan Page Client

const API = {
  posts: '/api/posts',
  subscribe: '/api/subscribe',
};

// ── Cookie helpers ──

function getCookie(name) {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
  return match ? match[2] : null;
}

function isRoyalMember() {
  return getCookie('royal_member') === '1';
}

function isUnlocked(post) {
  if (isRoyalMember()) return true;
  return getCookie(`unlocked_${post.id}`) === '1';
}

// ── Escape ──

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// ── State ──

let posts = [];
let likedPosts = new Set();

// ── Render ──

async function loadContent() {
  const feed = document.getElementById('feed');

  try {
    const res = await fetch(API.posts);
    if (!res.ok) throw new Error('Failed to load');
    posts = await res.json();

    const member = isRoyalMember();

    // Update profile stats
    document.getElementById('stat-posts').textContent = posts.length;

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

function renderPost(post, isMember) {
  const mediaUrl = `/uploads/${post.filename}`;
  const thumbnailUrl = post.thumbnail && post.thumbnail !== post.filename
    ? `/uploads/${post.thumbnail}`
    : mediaUrl;
  const date = new Date(post.created_at).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric'
  });
  const isLiked = likedPosts.has(post.id);

  const unlocked = isUnlocked(post);
  let mediaHtml;
  if (post.locked && !unlocked) {
    // Blurred preview with buy options
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
          <button class="post-locked-sub" onclick="openSubscribeModal()">👑 Word Royal Member — €4,99/maand</button>
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
        <button class="action-btn action-btn-like ${isLiked ? 'liked' : ''}" data-post-id="${post.id}">
          <span class="action-icon">${isLiked ? '❤️' : '🤍'}</span>
          <span>${isLiked ? '1' : '0'}</span>
        </button>
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
}

// ── Like toggle ──

function toggleLike(btn) {
  const postId = parseInt(btn.dataset.postId);
  const isLiked = likedPosts.has(postId);
  if (isLiked) {
    likedPosts.delete(postId);
  } else {
    likedPosts.add(postId);
  }
  // Re-render just the like button state
  const heart = btn.querySelector('.action-icon');
  const count = btn.querySelector('span:last-child');
  if (likedPosts.has(postId)) {
    btn.classList.add('liked');
    heart.textContent = '❤️';
    count.textContent = '1';
  } else {
    btn.classList.remove('liked');
    heart.textContent = '🤍';
    count.textContent = '0';
  }
}

// ── Purchase state ──

let purchaseMode = 'subscription'; // 'subscription' | 'item'
let purchasePostId = null;
let purchasePrice = '4.99';
let purchaseLabel = '€4,99/maand';

function buyItem(postId, mediaType) {
  purchaseMode = 'item';
  purchasePostId = postId;
  const price = mediaType === 'video' ? '0.20' : '0.05';
  purchasePrice = price;
  const label = mediaType === 'video' ? '€0,20' : '€0,05';
  purchaseLabel = label;
  document.getElementById('modal-icon').textContent = '🔓';
  document.getElementById('modal-title').textContent = 'Ontgrendel deze content';
  document.getElementById('modal-subtitle').innerHTML = `Betaal <strong>${label}</strong> en krijg direct toegang`;
  document.getElementById('modal-btn-text').textContent = `Betaal ${label} met iDeal`;
  document.getElementById('modal-note').textContent = '🔒 Parodie — er wordt niks afgeschreven';
  openSubscribeModal();
}

// ── Subscribe modal ──

function openSubscribeModal() {
  if (purchaseMode === 'subscription') {
    document.getElementById('modal-icon').textContent = '👑';
    document.getElementById('modal-title').textContent = 'Word Royal Member';
    document.getElementById('modal-subtitle').innerHTML = 'Onbeperkt toegang voor <strong>€4,99/maand</strong>';
    document.getElementById('modal-btn-text').textContent = 'Betaal €4,99 met iDeal';
    document.getElementById('modal-note').textContent = '🔒 Parodie — er wordt niks afgeschreven';
  }
  document.getElementById('subscribe-modal').style.display = 'flex';
}

function closeSubscribeModal() {
  document.getElementById('subscribe-modal').style.display = 'none';
}

function updateMemberUI() {
  const isMember = isRoyalMember();
  document.getElementById('btn-subscribe-hero').style.display = isMember ? 'none' : '';
  document.getElementById('btn-subscribed-hero').style.display = isMember ? 'inline-flex' : 'none';
}

async function handleSubscribeSubmit(e) {
  e.preventDefault();
  closeSubscribeModal();

  const bank = document.getElementById('bank-select').value;
  if (!bank) {
    showToast('❌ Selecteer een bank om te betalen.', null, true);
    return;
  }

  const body = purchaseMode === 'item' && purchasePostId
    ? JSON.stringify({ post_id: purchasePostId })
    : JSON.stringify({});

  try {
    const res = await fetch(API.subscribe, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body,
    });

    if (!res.ok) throw new Error('Purchase failed');
    const data = await res.json();

    const prefix = purchaseMode === 'item' ? '🏦 Gepind!' : '👑';
    const charged = purchasePrice ? `€${purchasePrice}`.replace('.', ',') : null;
    showToast(`${prefix} ${data.message}`, charged);

    // Reset state after purchase
    purchaseMode = 'subscription';
    purchasePostId = null;
    purchasePrice = '4.99';

    updateMemberUI();
    loadContent();
  } catch (err) {
    purchaseMode = 'subscription';
    purchasePostId = null;
    purchasePrice = '4.99';
    showToast('❌ Betaling mislukt! Eigenlijk niks aan de hand — probeer opnieuw.', null, true);
  }
}

// ── Toast ──

function showToast(message, amount, isError) {
  const container = document.getElementById('toast-container');
  const toast = document.createElement('div');
  toast.className = 'toast';

  if (isError) {
    toast.innerHTML = `
      <div class="toast-icon">❌</div>
      <div class="toast-text">${message}</div>
    `;
  } else {
    toast.innerHTML = `
      <div class="toast-icon">✅</div>
      <div class="toast-text">${message} <span class="amount">${amount}</span></div>
    `;
  }

  container.appendChild(toast);
  setTimeout(() => {
    toast.style.transition = 'opacity 0.5s';
    toast.style.opacity = '0';
    setTimeout(() => toast.remove(), 500);
  }, 4000);
}

// ── Umami tracking ──

async function initUmamiTracking() {
  try {
    const res = await fetch('/api/settings/analytics');
    if (!res.ok) return;
    const settings = await res.json();
    if (!settings.tracking_enabled || !settings.umami_script_url || !settings.umami_website_id) return;

    // Auto-append /script.js if URL doesn't end with a .js file
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

// ── Tab Switching ──

let currentTab = 'posts';

function switchTab(tab) {
  currentTab = tab;
  document.querySelectorAll('.tab').forEach(t => {
    t.classList.toggle('tab-active', t.dataset.tab === tab);
  });
  const feed = document.getElementById('feed');
  feed.className = 'feed' + (tab === 'media' ? ' feed-media' : '');

  if (tab === 'media') {
    renderMediaView();
  } else {
    renderPostsView();
  }
}

function renderPostsView() {
  const feed = document.getElementById('feed');
  if (posts.length === 0) {
    feed.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">🍳</div>
        <p>The Chef is still cooking... Check back soon for some sizzling content!</p>
      </div>
    `;
    return;
  }
  feed.innerHTML = posts.map(post => renderPost(post)).join('');

  document.querySelectorAll('.action-btn-like').forEach(btn => {
    btn.addEventListener('click', () => toggleLike(btn));
  });
}

function renderMediaView() {
  const feed = document.getElementById('feed');

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
    const unlocked = isUnlocked(post);
    const isLocked = post.locked && !unlocked;

    let mediaEl = post.media_type === 'video'
      ? `<video src="${mediaUrl}" preload="metadata"></video>`
      : `<img src="${mediaUrl}" alt="${escapeHtml(post.title)}" loading="lazy">`;

    if (isLocked) {
      mediaEl = `<div class="media-grid-locked"><div class="media-grid-blur">${mediaEl}</div><div class="media-grid-overlay" onclick="buyItem(${post.id}, '${post.media_type}')">🔒</div></div>`;
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

// ── Init ──

document.addEventListener('DOMContentLoaded', () => {
  initUmamiTracking();

  // Tab switching
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => switchTab(tab.dataset.tab));
  });

  // Hero subscribe button
  document.getElementById('btn-subscribe-hero').addEventListener('click', openSubscribeModal);

  // Modal close
  document.getElementById('modal-close').addEventListener('click', closeSubscribeModal);
  document.getElementById('subscribe-modal').addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeSubscribeModal();
  });

  // Subscribe form
  document.getElementById('subscribe-form').addEventListener('submit', handleSubscribeSubmit);

  // Init member UI state
  updateMemberUI();

  // Load content
  loadContent();
});
