const message = document.getElementById('message');
const rows = document.getElementById('materials');
let active = true;
window.addEventListener('auth-cleared', () => {
  active = false;
  rows.replaceChildren();
  document.getElementById('text').textContent = '';
  document.querySelectorAll('[data-class], #role').forEach(node => { node.textContent = ''; });
  document.getElementById('upload-card').hidden = true;
  document.getElementById('student-note').hidden = true;
  document.title = 'CampusClaw';
});
window.addEventListener('pageshow', event => { if (event.persisted) location.reload(); });

function check(response) {
  if (!response.ok) {
    const text = response.status === 403 ? '权限不足。' : response.status === 413 ? '文件超过大小限制。' : '操作失败，请重试。';
    throw new Error(text);
  }
  return response;
}
function showError(error) { if (active) message.textContent = error.message || '连接失败，请重试。'; }
async function readMaterial(id) {
  const response = check(await CampusAPI.request('/api/materials/' + encodeURIComponent(id)));
  const material = await response.json();
  if (active) document.getElementById('text').textContent = material.plain_text;
}
async function downloadMaterial(item) {
  const response = check(await CampusAPI.request('/api/materials/' + encodeURIComponent(item.id) + '/file'));
  const blob = await response.blob();
  if (!active) return;
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = item.original_name;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
async function loadMaterials() {
  const response = check(await CampusAPI.request('/api/materials'));
  const {items} = await response.json();
  if (!active) return;
  rows.replaceChildren();
  for (const item of items) {
    const row = document.createElement('tr');
    for (const value of [item.original_name, item.id, item.size, item.created_at, item.uploader]) {
      const cell = document.createElement('td');
      cell.textContent = value;
      row.append(cell);
    }
    const actions = document.createElement('td');
    actions.className = 'material-actions';
    for (const [label, action] of [['阅读文本', () => readMaterial(item.id)], ['下载', () => downloadMaterial(item)]]) {
      const button = document.createElement('button');
      button.type = 'button';
      button.textContent = label;
      button.addEventListener('click', () => action().catch(showError));
      actions.append(button);
    }
    row.append(actions);
    rows.append(row);
  }
  if (!items.length) {
    const row = document.createElement('tr');
    const cell = document.createElement('td');
    cell.colSpan = 6;
    cell.className = 'empty-state';
    cell.textContent = '本班暂无材料。';
    row.append(cell);
    rows.append(row);
  }
}
document.getElementById('logout').addEventListener('submit', event => {
  event.preventDefault();
  CampusAPI.logout();
});
document.getElementById('upload').addEventListener('submit', async event => {
  event.preventDefault();
  try {
    const response = check(await CampusAPI.request('/api/materials', {method: 'POST', body: new FormData(event.target)}));
    await response.json();
    if (!active) return;
    event.target.reset();
    message.textContent = '上传成功。';
    await loadMaterials();
  } catch (error) { showError(error); }
});
(async () => {
  if (!CampusAPI.hasToken()) { CampusAPI.clear(); location.replace('/login'); return; }
  const response = check(await CampusAPI.request('/api/auth/me'));
  const me = await response.json();
  if (!active) return;
  const label = me.class_name.endsWith('班') ? me.class_name : me.class_name + '班';
  document.querySelectorAll('[data-class]').forEach(node => { node.textContent = label; });
  document.title = label + ' · CampusClaw';
  document.getElementById('role').textContent = me.role === 'teacher' ? '教师' : '学生';
  document.getElementById('upload-card').hidden = me.role !== 'teacher';
  document.getElementById('student-note').hidden = me.role !== 'student';
  await loadMaterials();
})().catch(showError);
