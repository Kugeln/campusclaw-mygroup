const loginMessage = document.getElementById('message');
loginMessage.textContent = sessionStorage.getItem('campusclaw.notice') || '';
sessionStorage.removeItem('campusclaw.notice');
document.getElementById('login').addEventListener('submit', async event => {
  event.preventDefault();
  CampusAPI.clear();
  try {
    const response = await CampusAPI.request('/api/auth/login', {
      method: 'POST', body: new FormData(event.target),
    }, false);
    if (!response.ok) { loginMessage.textContent = '登录失败，请检查账号密码后重试。'; return; }
    const result = await response.json();
    if (result.token_type !== 'Bearer' || typeof result.access_token !== 'string' || !result.access_token) {
      throw new Error('Invalid login response');
    }
    CampusAPI.saveToken(result.access_token);
    location.assign('/materials');
  } catch { loginMessage.textContent = '连接失败或无法保存登录状态，请重试。'; }
});
