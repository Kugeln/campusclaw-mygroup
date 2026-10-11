// Only this module sends API requests. Credentials never enter URLs or cookies.
(() => {
  const key = 'campusclaw.token';
  let generation = 0;
  function clear() {
    generation += 1;
    sessionStorage.removeItem(key);
    window.dispatchEvent(new Event('auth-cleared'));
  }
  function toLogin() { location.replace('/login'); }
  async function request(path, options = {}, authenticated = true) {
    const url = new URL(path, location.origin);
    if (url.origin !== location.origin || !url.pathname.startsWith('/api/') || url.username || url.password) {
      throw new Error('Only same-origin API requests are allowed');
    }
    const headers = new Headers(options.headers);
    headers.delete('Authorization');
    if (authenticated) {
      const token = sessionStorage.getItem(key);
      if (!token) { clear(); toLogin(); throw new Error('Authentication required'); }
      headers.set('Authorization', `Bearer ${token}`);
    }
    const started = generation;
    const response = await fetch(url.href, {...options, headers, credentials: 'omit', redirect: 'error'});
    if (authenticated && response.status === 401) {
      clear(); toLogin(); throw new Error('Authentication required');
    }
    if (authenticated && started !== generation) throw new Error('Authentication changed');
    return response;
  }
  async function logout() {
    let confirmed = false;
    try { confirmed = (await request('/api/auth/logout', {method: 'POST'})).status === 204; }
    catch { /* Local state is cleared even if the server is unavailable. */ }
    finally {
      clear();
      if (!confirmed) sessionStorage.setItem('campusclaw.notice', '本地登录已清除，服务端退出未确认，旧 Token 可能仍有效。');
      toLogin();
    }
  }
  window.CampusAPI = Object.freeze({request, logout, clear,
    hasToken: () => Boolean(sessionStorage.getItem(key)),
    saveToken: token => { clear(); sessionStorage.setItem(key, token); },
  });
})();
