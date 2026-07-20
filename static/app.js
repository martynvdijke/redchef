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

    if (posts.length === 0) {
      feed.innerHTML = `
        <div class="empty-state">
          <div class="empty-icon">🍳</div>
          <p>The Chef is still cooking... Check back soon for some sizzling content!</p>
        </div>
      `;
      return;
    }

    feed.innerHTML = posts.map(post => renderPost(post, member)).join('');

    // Attach subscribe buttons
    document.querySelectorAll('.post-locked-btn').forEach(btn => {
      btn.addEventListener('click', () => openSubscribeModal());
    });

    // Attach like buttons
    document.querySelectorAll('.action-btn-like').forEach(btn => {
      btn.addEventListener('click', () => toggleLike(btn));
    });

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

  let mediaHtml;
  if (post.locked && !isMember) {
    // Blurred preview with lock overlay
    mediaHtml = `
      <div class="post-media-locked">
        <img class="post-media-blur" src="${thumbnailUrl}" alt="" loading="lazy"
             onerror="this.style.display='none';this.parentElement.style.background='#222'">
        <div class="post-locked-overlay">
          <div class="post-locked-icon">🔒</div>
          <div class="post-locked-text">Royal Members only</div>
          <button class="post-locked-btn">Subscribe to view</button>
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

// ── Subscribe modal ──

function openSubscribeModal() {
  document.getElementById('subscribe-modal').style.display = 'flex';
}

function closeSubscribeModal() {
  document.getElementById('subscribe-modal').style.display = 'none';
}

async function handleSubscribeSubmit(e) {
  e.preventDefault();
  closeSubscribeModal();

  // Show processing overlay
  const overlay = document.getElementById('processing-overlay');
  overlay.style.display = 'flex';

  // Brief processing simulation for the fake payment feel
  await new Promise(resolve => setTimeout(resolve, 1800));

  try {
    const res = await fetch(API.subscribe, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });

    overlay.style.display = 'none';

    if (!res.ok) throw new Error('Subscribe failed');
    const data = await res.json();

    showToast(data.message, data.charged);
    updateMemberUI();
    loadContent(); // Re-render feed unlocked
  } catch (err) {
    overlay.style.display = 'none';
    showToast('❌ Subscription failed! Actually, nothing went wrong — try again.', null, true);
  }
}

function updateMemberUI() {
  const isMember = isRoyalMember();
  document.getElementById('btn-subscribe-hero').style.display = isMember ? 'none' : '';
  document.getElementById('btn-subscribed-hero').style.display = isMember ? 'inline-flex' : 'none';
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
      <div class="toast-text">${message} <span class="amount">$${amount}</span></div>
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

    const script = document.createElement('script');
    script.async = true;
    script.defer = true;
    script.src = settings.umami_script_url;
    script.setAttribute('data-website-id', settings.umami_website_id);
    document.head.appendChild(script);
  } catch (_) {}
}

// ── Init ──

document.addEventListener('DOMContentLoaded', () => {
  initUmamiTracking();

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
