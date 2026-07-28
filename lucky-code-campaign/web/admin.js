const loginView = document.querySelector('#login-view');
const dashboardView = document.querySelector('#dashboard-view');
const loginForm = document.querySelector('#login-form');
const passwordInput = document.querySelector('#password');
const loginError = document.querySelector('#login-error');
const logoutButton = document.querySelector('#logout-button');
const refreshButton = document.querySelector('#refresh-button');
const codesInput = document.querySelector('#codes-input');
const fileInput = document.querySelector('#file-input');
const dropZone = document.querySelector('#drop-zone');
const lineCount = document.querySelector('#line-count');
const importButton = document.querySelector('#import-button');
const importFeedback = document.querySelector('#import-feedback');
const searchInput = document.querySelector('#search-input');
const recordsBody = document.querySelector('#records-body');
const recordsEmpty = document.querySelector('#records-empty');
const recordsSummary = document.querySelector('#records-summary');
const pageLabel = document.querySelector('#page-label');
const prevPage = document.querySelector('#prev-page');
const nextPage = document.querySelector('#next-page');
const exportLink = document.querySelector('#export-link');
const toast = document.querySelector('#toast');

const state = { page: 1, pageSize: 20, total: 0, query: '' };
let searchTimer;
let toastTimer;

async function request(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(payload.error || '请求失败，请稍后再试');
    error.status = response.status;
    error.code = payload.code;
    throw error;
  }
  return payload;
}

function showLogin() {
  dashboardView.hidden = true;
  loginView.hidden = false;
  passwordInput.focus();
}

function showDashboard() {
  loginView.hidden = true;
  dashboardView.hidden = false;
}

function showToast(message) {
  clearTimeout(toastTimer);
  toast.textContent = message;
  toast.classList.add('is-visible');
  toastTimer = setTimeout(() => toast.classList.remove('is-visible'), 2200);
}

function formatDate(value) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(new Date(value));
}

function setStats(stats) {
  document.querySelector('#stat-total').textContent = stats.total.toLocaleString('zh-CN');
  document.querySelector('#stat-claimed').textContent = stats.claimed.toLocaleString('zh-CN');
  document.querySelector('#stat-remaining').textContent = stats.remaining.toLocaleString('zh-CN');
  const rate = stats.total ? Math.round((stats.claimed / stats.total) * 100) : 0;
  document.querySelector('#claim-rate').textContent = `领取率 ${rate}%`;
}

async function loadStats() {
  const stats = await request('/gift/api/admin/stats');
  setStats(stats);
  return stats;
}

function createCopyButton(code) {
  const button = document.createElement('button');
  button.className = 'icon-button row-copy';
  button.type = 'button';
  button.title = '复制兑换码';
  button.setAttribute('aria-label', '复制兑换码');
  button.innerHTML = '<svg aria-hidden="true" viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
  button.addEventListener('click', async () => {
    await copyText(code);
    showToast('兑换码已复制');
  });
  return button;
}

function renderRecords(payload) {
  recordsBody.replaceChildren();
  for (const claim of payload.items) {
    const row = document.createElement('tr');
    const emailCell = document.createElement('td');
    const codeCell = document.createElement('td');
    const dateCell = document.createElement('td');
    const actionCell = document.createElement('td');
    const code = document.createElement('code');
    emailCell.textContent = claim.email;
    code.textContent = claim.code;
    codeCell.appendChild(code);
    dateCell.textContent = formatDate(claim.claimedAt);
    actionCell.appendChild(createCopyButton(claim.code));
    row.append(emailCell, codeCell, dateCell, actionCell);
    recordsBody.appendChild(row);
  }

  state.total = payload.total;
  const pages = Math.max(1, Math.ceil(payload.total / state.pageSize));
  recordsEmpty.hidden = payload.items.length !== 0;
  recordsSummary.textContent = `共 ${payload.total.toLocaleString('zh-CN')} 条`;
  pageLabel.textContent = `${state.page} / ${pages}`;
  prevPage.disabled = state.page <= 1;
  nextPage.disabled = state.page >= pages;
  exportLink.href = `/gift/api/admin/claims/export?q=${encodeURIComponent(state.query)}`;
}

async function loadRecords() {
  const params = new URLSearchParams({
    page: String(state.page), pageSize: String(state.pageSize), q: state.query,
  });
  const payload = await request(`/gift/api/admin/claims?${params}`);
  renderRecords(payload);
}

async function loadDashboard() {
  try {
    await Promise.all([loadStats(), loadRecords()]);
    showDashboard();
  } catch (error) {
    if (error.status === 401) showLogin();
    else showToast(error.message);
  }
}

function updateLineCount() {
  const count = codesInput.value.split(/\r?\n/).filter((line) => line.trim()).length;
  lineCount.textContent = `${count.toLocaleString('zh-CN')} 行`;
}

async function readFile(file) {
  if (!file) return;
  if (file.size > 5 * 1024 * 1024) {
    showToast('文件不能超过 5 MB');
    return;
  }
  codesInput.value = await file.text();
  updateLineCount();
  importFeedback.textContent = `已读取 ${file.name}`;
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

loginForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  loginError.textContent = '';
  const button = loginForm.querySelector('button');
  button.disabled = true;
  try {
    await request('/gift/api/admin/login', {
      method: 'POST', body: JSON.stringify({ password: passwordInput.value }),
    });
    passwordInput.value = '';
    await loadDashboard();
  } catch (error) {
    loginError.textContent = error.message;
  } finally {
    button.disabled = false;
  }
});

logoutButton.addEventListener('click', async () => {
  await request('/gift/api/admin/logout', { method: 'POST', body: '{}' }).catch(() => {});
  showLogin();
});

refreshButton.addEventListener('click', async () => {
  refreshButton.disabled = true;
  try {
    await Promise.all([loadStats(), loadRecords()]);
    showToast('数据已刷新');
  } catch (error) {
    showToast(error.message);
  } finally {
    refreshButton.disabled = false;
  }
});

codesInput.addEventListener('input', updateLineCount);
fileInput.addEventListener('change', () => readFile(fileInput.files[0]));
['dragenter', 'dragover'].forEach((type) => dropZone.addEventListener(type, () => dropZone.classList.add('is-dragging')));
['dragleave', 'drop'].forEach((type) => dropZone.addEventListener(type, () => dropZone.classList.remove('is-dragging')));

importButton.addEventListener('click', async () => {
  if (!codesInput.value.trim()) {
    showToast('请先粘贴或上传兑换码');
    codesInput.focus();
    return;
  }
  importButton.disabled = true;
  importFeedback.textContent = '正在导入…';
  try {
    const payload = await request('/gift/api/admin/codes/import', {
      method: 'POST', body: JSON.stringify({ content: codesInput.value }),
    });
    importFeedback.textContent = `成功新增 ${payload.added} 枚，跳过 ${payload.duplicates} 枚重复码`;
    codesInput.value = '';
    fileInput.value = '';
    updateLineCount();
    setStats(payload.stats);
  } catch (error) {
    importFeedback.textContent = error.message;
  } finally {
    importButton.disabled = false;
  }
});

searchInput.addEventListener('input', () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(async () => {
    state.query = searchInput.value.trim();
    state.page = 1;
    try { await loadRecords(); } catch (error) { showToast(error.message); }
  }, 280);
});

prevPage.addEventListener('click', async () => {
  if (state.page <= 1) return;
  state.page -= 1;
  await loadRecords();
});

nextPage.addEventListener('click', async () => {
  const pages = Math.max(1, Math.ceil(state.total / state.pageSize));
  if (state.page >= pages) return;
  state.page += 1;
  await loadRecords();
});

loadDashboard();
