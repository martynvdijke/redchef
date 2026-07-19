// RedChef - Fan Page Client

const API = {
  posts: '/api/posts',
  unlock: '/api/unlock',
};

// Cookie helpers
function getCookie(name) {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
  return match ? match[2] : null;
}

function getUnlockedIds() {
  const val = getCookie('unlocked_posts');
  if (!val) return [];
  return val.split(',').map(Number).filter(n => !isNaN(n));
}

// Render
async function loadContent() {
  const grid = document.getElementById('content-grid');
  const sectionTitle = document.getElementById('section-title');

  try {
    const res = await fetch(API.posts);
    if (!res.ok) throw new Error('Failed to load');
    const posts = await res.json();

    if (posts.length === 0) {
      grid.innerHTML = `
        <div class="empty-state">
          <div class="big-icon">🍳</div>
          <p>No content yet. The Chef is cooking up something special!</p>
        </div>
      `;
      sectionTitle.textContent = 'Latest from The Chef';
      return;
    }

    sectionTitle.textContent = `Latest from The Chef — ${posts.length} post${posts.length !== 1 ? 's' : ''}`;

    const unlocked = getUnlockedIds();
    grid.innerHTML = posts.map(post => renderCard(post, unlocked.includes(post.id))).join('');

    // Attach unlock handlers
    document.querySelectorAll('.unlock-btn').forEach(btn => {
      btn.addEventListener('click', () => unlockPost(parseInt(btn.dataset.postId)));
    });

  } catch (err) {
    grid.innerHTML = `
      <div class="empty-state">
        <div class="big-icon">😰</div>
        <p>Couldn't reach the Chef's kitchen. Try again later!</p>
      </div>
    `;
  }
}

function renderCard(post, isUnlocked) {
  const mediaUrl = `/uploads/${post.filename}`;
  const thumbnailUrl = post.thumbnail ? `/uploads/${post.thumbnail}` : mediaUrl;
  const date = new Date(post.created_at).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric'
  });

  if (post.locked && !isUnlocked) {
    return `
      <div class="card card-locked">
        <div style="position:relative;">
          <img class="card-image" src="${thumbnailUrl}" alt="${escapeHtml(post.title)}" loading="lazy"
               onerror="this.parentElement.innerHTML='<div class=card-image-placeholder>🍳</div>'">
          <div class="card-locked-overlay">
            <div class="lock-icon">🔒</div>
            <div class="price-tag">$0.05</div>
            <button class="unlock-btn" data-post-id="${post.id}">Pay 5¢ to see this</button>
          </div>
        </div>
        <div class="card-body">
          <div class="card-title">${escapeHtml(post.title)}</div>
          ${post.description ? `<div class="card-desc">${escapeHtml(post.description)}</div>` : ''}
          <div class="card-date">${date}</div>
        </div>
      </div>
    `;
  }

  return `
    <div class="card">
      ${post.media_type === 'video'
        ? `<video class="card-image" controls preload="metadata">
             <source src="${mediaUrl}" type="video/mp4">
           </video>`
        : `<img class="card-image" src="${mediaUrl}" alt="${escapeHtml(post.title)}" loading="lazy"
               onerror="this.parentElement.innerHTML='<div class=card-image-placeholder>🍳</div>'">`
      }
      <div class="card-body">
        <div class="card-title">${escapeHtml(post.title)}</div>
        ${post.description ? `<div class="card-desc">${escapeHtml(post.description)}</div>` : ''}
        <div class="card-date">${date}</div>
      </div>
    </div>
  `;
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// Unlock flow
function unlockPost(postId) {
  const overlay = document.createElement('div');
  overlay.className = 'processing-overlay';
  overlay.innerHTML = `
    <div class="spinner">💳</div>
    <div class="processing-text">PROCESSING PAYMENT...</div>
  `;
  document.body.appendChild(overlay);

  // Simulate payment processing
  setTimeout(async () => {
    try {
      const res = await fetch(API.unlock, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ post_id: postId }),
      });

      if (!res.ok) throw new Error('Unlock failed');
      const data = await res.json();

      overlay.remove();
      showToast(data.message, data.charged);
      loadContent(); // Re-render grid
    } catch (err) {
      overlay.remove();
      showToast('❌ Payment failed! Actually, nothing went wrong — try again.', null, true);
    }
  }, 2000);
}

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

// Init
document.addEventListener('DOMContentLoaded', loadContent);
