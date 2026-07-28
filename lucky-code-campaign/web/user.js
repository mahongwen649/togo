const form = document.querySelector('#claim-form');
const claimError = document.querySelector('#claim-error');
const claimButton = document.querySelector('#claim-button');
const lastClaim = document.querySelector('#last-claim');
const lastClaimCode = document.querySelector('#last-claim-code');
const copyLastClaimButton = document.querySelector('#copy-last-claim-button');
const formView = document.querySelector('#claim-form-view');
const drawView = document.querySelector('#draw-view');
const shuffleCode = document.querySelector('#shuffle-code');
const resultView = document.querySelector('#result-view');
const emptyView = document.querySelector('#empty-view');
const resultKicker = document.querySelector('#result-kicker');
const resultCode = document.querySelector('#result-code');
const copyButton = document.querySelector('#copy-button');
const anotherButton = document.querySelector('#use-another-button');
const emptyQueryButton = document.querySelector('#empty-query-button');
const copyQqButton = document.querySelector('#copy-qq-button');
const poolPill = document.querySelector('#pool-pill');
const poolStatus = document.querySelector('#pool-status');
const toast = document.querySelector('#toast');
const claimPanel = document.querySelector('.claim-panel');

let toastTimer;
let shuffleTimer;
let cooldownTimer;
let isClaimLoading = false;
let cooldownUntil = 0;
const claimCooldownSeconds = 60;
const cooldownStorageKey = 'gift-claim-cooldown-until';
const lastClaimStorageKey = 'gift-last-claim-code';
const drawDuration = window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 150 : 1700;

async function request(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(payload.error || '请求失败，请稍后再试');
    error.code = payload.code;
    error.retryAfterSeconds = payload.retryAfterSeconds;
    throw error;
  }
  return payload;
}

async function loadStatus() {
  try {
    const status = await request('/gift/api/status');
    if (status.available) {
      poolStatus.textContent = `奖池剩余 ${status.remaining} 枚`;
      poolPill.classList.remove('is-empty');
    } else {
      poolStatus.textContent = '本轮已抢完';
      poolPill.classList.add('is-empty');
    }
  } catch {
    poolStatus.textContent = '奖池状态暂不可用';
  }
}

function getCooldownSeconds() {
  return Math.max(0, Math.ceil((cooldownUntil - Date.now()) / 1000));
}

function renderClaimButton() {
  const remaining = getCooldownSeconds();
  claimButton.disabled = isClaimLoading || remaining > 0;
  claimButton.dataset.state = isClaimLoading ? 'loading' : remaining > 0 ? 'cooldown' : 'ready';

  if (isClaimLoading) {
    claimButton.querySelector('span').textContent = '正在抽取…';
    return;
  }
  if (remaining > 0) {
    const minutes = Math.floor(remaining / 60).toString().padStart(2, '0');
    const seconds = (remaining % 60).toString().padStart(2, '0');
    claimButton.querySelector('span').textContent = `请等待 ${minutes}:${seconds}`;
    return;
  }
  claimButton.querySelector('span').textContent = '立即拼手气';
}

function clearStoredCooldown() {
  try {
    localStorage.removeItem(cooldownStorageKey);
  } catch {
    // Storage can be unavailable in privacy mode; the server still enforces cooldown.
  }
}

function startCooldown(seconds) {
  const safeSeconds = Math.max(0, Number(seconds) || 0);
  cooldownUntil = Date.now() + safeSeconds * 1000;
  clearInterval(cooldownTimer);

  if (safeSeconds > 0) {
    try {
      localStorage.setItem(cooldownStorageKey, String(cooldownUntil));
    } catch {
      // Keep the in-memory countdown when storage is unavailable.
    }
    cooldownTimer = setInterval(() => {
      renderClaimButton();
      if (getCooldownSeconds() === 0) {
        clearInterval(cooldownTimer);
        clearStoredCooldown();
      }
    }, 1000);
  } else {
    clearStoredCooldown();
  }
  renderClaimButton();
}

function restoreCooldown() {
  try {
    const storedUntil = Number(localStorage.getItem(cooldownStorageKey));
    const remaining = Math.ceil((storedUntil - Date.now()) / 1000);
    if (remaining > 0) {
      startCooldown(Math.min(claimCooldownSeconds, remaining));
      return;
    }
  } catch {
    // Ignore unavailable or invalid browser storage.
  }
  startCooldown(0);
}

function showLastClaim(code) {
  const normalizedCode = typeof code === 'string' ? code.trim() : '';
  if (!normalizedCode) return;
  lastClaimCode.textContent = normalizedCode;
  lastClaim.hidden = false;
}

function storeLastClaim(code) {
  showLastClaim(code);
  try {
    localStorage.setItem(lastClaimStorageKey, code);
  } catch {
    // The successful result remains visible even when storage is unavailable.
  }
}

function restoreLastClaim() {
  try {
    showLastClaim(localStorage.getItem(lastClaimStorageKey));
  } catch {
    // Ignore unavailable browser storage.
  }
}

function setLoading(loading) {
  isClaimLoading = loading;
  renderClaimButton();
}

function showResult(payload) {
  endDraw();
  formView.hidden = true;
  emptyView.hidden = true;
  resultView.hidden = false;
  resultKicker.textContent = '领取成功';
  resultCode.textContent = payload.claim.code;
  launchConfetti();
  resultView.focus?.();
}

function showEmpty() {
  endDraw();
  formView.hidden = true;
  resultView.hidden = true;
  emptyView.hidden = false;
}

function resetForm() {
  endDraw();
  resultView.hidden = true;
  emptyView.hidden = true;
  formView.hidden = false;
  claimError.textContent = '';
  claimButton.focus();
}

function beginDraw() {
  formView.hidden = true;
  resultView.hidden = true;
  emptyView.hidden = true;
  drawView.hidden = false;
  const masks = ['XXXX · XXXX · XXXX', '7KX9 · ???? · 2PQM', 'LUCK · ---- · CODE', '9R3A · 6K2M · ????'];
  let index = 0;
  shuffleCode.textContent = masks[index];
  clearInterval(shuffleTimer);
  shuffleTimer = setInterval(() => {
    index = (index + 1) % masks.length;
    shuffleCode.textContent = masks[index];
  }, 180);
}

function endDraw() {
  clearInterval(shuffleTimer);
  drawView.hidden = true;
}

function restoreFormAfterError() {
  endDraw();
  formView.hidden = false;
}

function launchConfetti() {
  const panel = document.querySelector('.claim-panel');
  const colors = ['#f0c84b', '#14b8a6', '#ef6a62', '#172121', '#ffffff'];
  for (let index = 0; index < 18; index += 1) {
    const piece = document.createElement('i');
    piece.className = 'confetti-piece';
    piece.style.setProperty('--confetti-x', `${10 + Math.random() * 80}%`);
    piece.style.setProperty('--confetti-shift', `${Math.round((Math.random() - 0.5) * 130)}px`);
    piece.style.setProperty('--confetti-delay', `${Math.random() * 180}ms`);
    piece.style.setProperty('--confetti-color', colors[index % colors.length]);
    panel.appendChild(piece);
    setTimeout(() => piece.remove(), 1300);
  }
}

function showToast(message) {
  clearTimeout(toastTimer);
  toast.textContent = message;
  toast.classList.add('is-visible');
  toastTimer = setTimeout(() => toast.classList.remove('is-visible'), 2200);
}

async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const area = document.createElement('textarea');
  area.value = text;
  area.style.position = 'fixed';
  area.style.opacity = '0';
  document.body.appendChild(area);
  area.select();
  document.execCommand('copy');
  area.remove();
}

function showPanelRipple(event) {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  const rect = claimPanel.getBoundingClientRect();
  const size = Math.hypot(rect.width, rect.height) * 2;
  const ripple = document.createElement('span');
  ripple.className = 'click-ripple';
  ripple.style.width = `${size}px`;
  ripple.style.height = `${size}px`;
  ripple.style.left = `${event.clientX - rect.left}px`;
  ripple.style.top = `${event.clientY - rect.top}px`;
  const primaryRing = document.createElement('span');
  primaryRing.className = 'ripple-ring';
  const secondaryRing = document.createElement('span');
  secondaryRing.className = 'ripple-ring ripple-ring-secondary';
  const drop = document.createElement('span');
  drop.className = 'ripple-drop';
  ripple.append(primaryRing, secondaryRing, drop);
  claimPanel.appendChild(ripple);
  ripple.addEventListener('animationend', () => ripple.remove(), { once: true });
}

claimPanel.addEventListener('pointerdown', showPanelRipple);

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  claimError.textContent = '';
  setLoading(true);
  beginDraw();
  try {
    const [payload] = await Promise.all([
      request('/gift/api/claim', {
        method: 'POST',
        body: JSON.stringify({}),
      }),
      new Promise((resolve) => setTimeout(resolve, drawDuration)),
    ]);
    storeLastClaim(payload.claim.code);
    startCooldown(claimCooldownSeconds);
    showResult(payload);
    loadStatus();
  } catch (error) {
    if (error.code === 'pool_empty') {
      showEmpty();
      loadStatus();
    } else {
      restoreFormAfterError();
      if (error.code === 'claim_cooldown' && error.retryAfterSeconds) {
        startCooldown(error.retryAfterSeconds);
        const minutes = Math.floor(error.retryAfterSeconds / 60);
        const seconds = error.retryAfterSeconds % 60;
        claimError.textContent = `本设备还需等待 ${minutes} 分 ${seconds} 秒`;
      } else {
        claimError.textContent = error.message;
      }
    }
  } finally {
    setLoading(false);
  }
});

copyButton.addEventListener('click', async () => {
  try {
    await copyText(resultCode.textContent);
    showToast('兑换码已复制');
  } catch {
    showToast('复制失败，请手动选择兑换码');
  }
});

copyLastClaimButton.addEventListener('click', async () => {
  try {
    await copyText(lastClaimCode.textContent);
    showToast('兑换码已复制');
  } catch {
    showToast('复制失败，请手动选择兑换码');
  }
});

anotherButton.addEventListener('click', resetForm);
emptyQueryButton.addEventListener('click', resetForm);
copyQqButton.addEventListener('click', async () => {
  try {
    await copyText('921331486');
    showToast('QQ群号已复制');
  } catch {
    showToast('群号：921331486');
  }
});
restoreCooldown();
restoreLastClaim();
loadStatus();
