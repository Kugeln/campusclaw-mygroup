(() => {
  const $ = id => document.getElementById(id);
  const status = {ok:'成功',denied:'已拒绝 / 不可用',error:'调用失败'};
  // Both live meta and persisted configuration snapshots use the same safe view.
  function renderCalls(target, calls) {
    target.replaceChildren(); target.hidden = true;
    if (!Array.isArray(calls)) return;
    const valid = calls.filter(c => c && typeof c.ref === 'string' && typeof c.text === 'string' && Object.hasOwn(status,c.status));
    if (!valid.length) return;
    target.hidden = false;
    const title = document.createElement('strong'); title.textContent = '工具调用（外部数据）'; target.append(title);
    for (const call of valid) {
      const row = document.createElement('div'); row.className = 'tool-call'; row.dataset.status = call.status;
      const name = document.createElement('span'), text = document.createElement('p');
      name.textContent = call.ref+' · '+status[call.status]; text.textContent = call.text;
      row.append(name,text); target.append(row);
    }
  }
  let enabled = false, generation = 0, editing = null, started = false, pending = false;
  const controllers = new Set();
  const errors = {400:'配置无效，请检查名称、URL 和凭据。',401:'登录已失效，请重新登录。',403:'仅本班教师可管理工具服务。',404:'该服务不存在或不可访问，请刷新列表。',502:'连接工具服务失败，请检查服务配置或稍后重试。',503:'服务暂不可用；如该服务已有审计，不能删除，记录仍会保留。'};
  async function request(path, method = 'GET', value) {
    const controller = new AbortController(), version = generation;
    controllers.add(controller);
    const timer = setTimeout(() => controller.abort(),15000);
    try {
      const response = await CampusAPI.request('/api/mcp/'+path,{method,signal:controller.signal,...(value === undefined ? {} : {headers:{'Content-Type':'application/json'},body:JSON.stringify(value)})});
      if (!enabled || version !== generation) throw new Error('登录状态已改变。');
      if (!response.ok) throw new Error(errors[response.status] || '请求失败，请稍后重试。');
      return response.status === 204 ? null : await response.json();
    } catch(e) {
      // Never display fetch/remote error text (may contain internal addresses).
      if (e.name === 'AbortError') throw new Error('请求超时或已取消，请稍后重试。');
      if (Object.values(errors).includes(e.message) || e.message === '登录状态已改变。' || e.message === '请求失败，请稍后重试。') throw e;
      throw new Error('网络不可用，请检查连接后重试。');
    } finally { clearTimeout(timer); controllers.delete(controller); }
  }
  const say = text => { if (enabled) $('mcp-notice').textContent = text; };
  function text(tag, value) { const node = document.createElement(tag); node.textContent = value; return node; }
  function button(label, action) {
    const node = text('button',label); node.type = 'button';
    node.addEventListener('click', async () => {
      if (node.disabled || !enabled) return;
      node.disabled = true; say('处理中…');
      try { await action(); } catch(e) { say(e.message); } finally { node.disabled = false; }
    }); return node;
  }
  function edit(server) {
    editing = server || null; $('mcp-form').reset(); $('mcp-form-error').textContent = '';
    $('mcp-editor-title').textContent = server ? '编辑服务' : '新增服务';
    $('mcp-name').value = server?.name || ''; $('mcp-url').value = server?.base_url || '';
    $('mcp-token').required = false; $('mcp-editor').showModal(); $('mcp-name').focus();
  }
  async function loadServers() {
    $('mcp-servers').setAttribute('aria-busy','true'); say('正在读取服务…');
    try {
      const {servers} = await request('servers');
      $('mcp-servers').replaceChildren();
      if (!servers.length) $('mcp-servers').append(text('p','本班暂无 MCP 服务。'));
      for (const server of servers) {
        const card = document.createElement('article'); card.className = 'mcp-server'; card.dataset.serverId = server.id;
        card.append(text('h3',server.name),text('p',server.base_url),text('p',(server.connected ? '已连接' : '已断开')+' · 工具 '+server.tool_count+' · '+(server.has_auth ? '已配置凭据' : '未配置凭据')));
        const actions = document.createElement('div'); actions.className = 'mcp-actions';
        const tools = document.createElement('div'); tools.className = 'mcp-tools';
        actions.append(button('编辑',() => edit(server)),button('连接',async () => { await request('servers/'+server.id+'/connect','POST'); await loadServers(); say('已连接并更新工具列表。'); }),button('断开',async () => { await request('servers/'+server.id+'/disconnect','POST'); await loadServers(); say('服务已断开，后续工具调用将被拒绝。'); }),button('查看工具',async () => {
          tools.textContent = '正在读取工具…';
          try {
            const result = await request('servers/'+server.id+'/tools'); tools.replaceChildren();
            if (!result.tools.length) tools.append(text('p','暂无已发现工具，请先连接服务。'));
            for (const tool of result.tools) { const detail = document.createElement('details'); detail.append(text('summary',tool.name),text('p',tool.description),text('pre',JSON.stringify(tool.inputSchema,null,2))); tools.append(detail); }
            say('工具列表已更新。');
          } catch(e) { tools.replaceChildren(); throw e; }
        }),button('删除',async () => {
          if (!confirm('确认删除服务“'+server.name+'”？已有审计的服务不能删除；相关旧技能引用需手动更新。')) { say('已取消删除。'); return; }
          await request('servers/'+server.id,'DELETE'); await loadServers(); say('服务已删除。');
        })); card.append(actions,tools); $('mcp-servers').append(card);
      }
      say('服务列表已更新。');
    } finally { $('mcp-servers').removeAttribute('aria-busy'); }
  }
  async function loadAudits() {
    $('mcp-audits').setAttribute('aria-busy','true');
    try {
      const limit = $('mcp-limit').value === '100' ? 100 : 50;
      const {calls} = await request('tool-calls'+(limit === 50 ? '' : '?limit=100'));
      $('mcp-audits').replaceChildren();
      if (!calls.length) $('mcp-audits').append(text('p','本班暂无工具调用记录。'));
      for (const call of calls) {
        const row = document.createElement('article'); row.className = 'mcp-audit';
        row.append(text('strong',call.tool_ref+' · '+(status[call.status] || '状态未知')),text('p',call.created_at+' · 用户 '+call.user_id+(call.conversation_id ? ' · 会话 '+call.conversation_id : '')),text('p',call.detail)); $('mcp-audits').append(row);
      }
    } finally { $('mcp-audits').removeAttribute('aria-busy'); }
  }
  $('mcp-form').addEventListener('submit',async event => {
    event.preventDefault(); if (!enabled || pending) return;
    const name = $('mcp-name').value.trim(), base = $('mcp-url').value.trim();
    let url; try { url = new URL(base); } catch { $('mcp-form-error').textContent = '请输入有效的 HTTP 或 HTTPS URL。'; return; }
    if (!/^[a-z][a-z0-9_-]{0,63}$/.test(name) || !['http:','https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) { $('mcp-form-error').textContent = '请检查名称及 URL，URL 不得包含用户名、密码、查询或片段。'; return; }
    pending = true; $('mcp-save').disabled = true;
    const payload = {name,base_url:base,auth_token:$('mcp-token').value};
    // The request owns the only transient copy after submission, including failure.
    $('mcp-token').value = '';
    try {
      await request('servers'+(editing ? '/'+editing.id : ''),editing ? 'PUT' : 'POST',payload);
      if (!enabled) return;
      $('mcp-editor').close(); await loadServers(); say('服务已保存；如名称改变，请更新并重新保存相关技能。');
    } catch(e) { if (enabled) $('mcp-form-error').textContent = e.message; }
    finally { payload.auth_token = ''; pending = false; $('mcp-save').disabled = false; }
  });
  $('mcp-cancel').addEventListener('click',() => $('mcp-editor').close());
  $('mcp-editor').addEventListener('close',() => { $('mcp-token').value = ''; editing = null; });
  $('mcp-editor').addEventListener('cancel',() => { $('mcp-token').value = ''; });
  $('mcp-new').addEventListener('click',() => { if (enabled) edit(null); });
  const refresh = async action => { if (!enabled) return; try { await action(); } catch(e) { say(e.message); } };
  $('mcp-refresh').addEventListener('click',() => refresh(loadServers));
  $('mcp-audit-refresh').addEventListener('click',() => refresh(loadAudits));
  $('mcp-limit').addEventListener('change',() => refresh(loadAudits));
  for (const id of ['config-tab','mcp-tab']) $(id).addEventListener('click',async () => {
    if (!enabled) return;
    const mcp = id === 'mcp-tab';
    $('teacher-config-panel').hidden = mcp; $('mcp-panel').hidden = !mcp;
    $('config-tab').setAttribute('aria-selected',String(!mcp)); $('mcp-tab').setAttribute('aria-selected',String(mcp));
    if (mcp && !started) { started = true; await refresh(async () => { await loadServers(); await loadAudits(); }); }
  });
  function clear() {
    enabled = false; generation++; started = false; controllers.forEach(c => c.abort());
    $('mcp-editor').close(); $('mcp-form').reset(); $('mcp-token').value = ''; editing = null;
    ['mcp-servers','mcp-audits','mcp-notice','mcp-form-error'].forEach(id => $(id).replaceChildren());
    $('mcp-panel').hidden = true;
  }
  window.addEventListener('auth-cleared',clear);
  window.addEventListener('pagehide',clear);
  window.CampusMCP = Object.freeze({init:() => { enabled = true; },renderCalls});
})();
