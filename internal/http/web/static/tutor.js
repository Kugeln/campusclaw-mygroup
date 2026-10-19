(() => {
  const $ = id => document.getElementById(id);
  let active = true, agents = [], skills = [], materials = [], editingSkill = null, sourceOriginal = null, selected = null, view = 0, stream = null, busy = false;
  const compact = window.matchMedia('(max-width: 600px)');
  const disclosure = $('conversation-disclosure');
  const adaptConversations = () => { disclosure.open = !compact.matches; };
  adaptConversations(); compact.addEventListener('change', adaptConversations);
  const say = text => { if (active) $('notice').textContent = text; };
  const labels = {generating:'草稿 · 正在回答', completed:'已完成', failed:'回答失败 · 未完成草稿', interrupted:'回答中断 · 未完成草稿'};
  async function json(path, options) {
    const response = await CampusAPI.request(path, options);
    const value = await response.json();
    if (!response.ok) throw new Error(response.status === 409 ? '该会话正在回答，请稍后重试。' : response.status === 403 ? '权限不足。' : response.status === 404 ? '资源不可用。' : value.error || '请求失败。');
    return value;
  }
  const write = (method, value) => ({method, headers:{'Content-Type':'application/json'}, body:JSON.stringify(value)});
  function stop() { if (stream) stream.abort(); }
  window.addEventListener('auth-cleared', () => {
    active = false; view++; stop(); selected = null;
    agents = []; skills = []; materials = []; editingSkill = null; sourceOriginal = null;
    ['messages','conversations','skills','agent-skills','agent-materials','edit-agent','agent','identity','source-text','notice'].forEach(id => $(id).replaceChildren());
    ['prompt','question','agent-name','skill-source','skill-file'].forEach(id => $(id).value = ''); $('teacher').hidden = true; $('student').hidden = true;
  });
  window.addEventListener('pagehide', stop);
  window.addEventListener('pageshow', event => { if (event.persisted) location.reload(); });
  $('logout').addEventListener('click', () => CampusAPI.logout());
  $('cancel').addEventListener('click', stop);
  function controls() {
    $('send-button').disabled = busy || !selected;
    $('create').disabled = busy || !agents.length;
    $('cancel').hidden = !busy;
  }
  async function sourceAction(source, download) {
    const path = '/api/materials/'+encodeURIComponent(source.material_id);
    if (!download) {
      // Validate current permission, then show the historical source snapshot.
      await json(path);
      if (active) { $('source-text').textContent = source.title+' · 切片 '+source.chunk_index+'\n'+source.chunk_text; $('source-text').focus(); }
      return;
    }
    const response = await CampusAPI.request(path+'/file');
    if (!response.ok) throw new Error('当前材料不可访问。');
    const blob = await response.blob(); if (!active) return;
    const url = URL.createObjectURL(blob), link = document.createElement('a');
    link.href = url; link.download = source.title; document.body.append(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  function message(m) {
    const node = document.createElement('article'); node.className = 'message'; node.dataset.state = m.state;
    if (m.id) node.dataset.id = m.id;
    const heading = document.createElement('strong'); heading.textContent = m.role === 'user' ? '你' : '学习助手';
    const text = document.createElement('p'); text.textContent = m.content;
    const state = document.createElement('span'); state.className = 'state';
    state.textContent = m.role === 'user' ? '' : (labels[m.state] || '状态未知')+(m.skill_name ? ' · '+m.skill_name : '');
    node.append(heading,text,state);
    const tools = document.createElement('div'); tools.className = 'tool-calls'; tools.hidden = true;
    if (m.role === 'assistant') CampusMCP.renderCalls(tools,m.configuration_snapshot?.tool_calls);
    node.append(tools);
    const sources = document.createElement('div'); sources.className = 'sources';
    const candidates = document.createElement('div'), actual = document.createElement('div');
    candidates.className = 'candidate-sources'; actual.className = 'actual-sources';
    function showSources(target, items, title, empty) {
      target.replaceChildren(); const label = document.createElement('span'); label.className = 'source-label'; label.textContent = title; target.append(label);
      if (!items.length) { const note = document.createElement('span'); note.textContent = empty; target.append(note); }
      for (const source of items) {
      for (const [label, download] of [[`[${source.number}] ${source.title} · 切片 ${source.chunk_index}`,false],['下载材料',true]]) {
        const button = document.createElement('button'); button.type = 'button'; button.textContent = label;
        button.addEventListener('click', () => sourceAction(source,download).catch(e => say(e.message))); target.append(button);
      }
      }
    }
    let skill = m.skill_name || '';
    const updateState = value => { node.dataset.state = value; state.textContent = (labels[value] || '状态未知')+(skill ? ' · '+skill : ''); };
    if (m.role === 'assistant') {
      const evidence = m.candidates ?? m.evidence_snapshot?.map((item,index) => ({...item,number:index+1})) ?? ((m.citations || []).length ? null : []);
      showSources(candidates,evidence || [],evidence ? '本轮候选来源（不代表已引用）' : '历史未记录候选来源',evidence ? '无候选来源' : '候选列表不可用');
      showSources(actual,m.state === 'completed' ? m.citations || [] : [],m.state === 'completed' ? '最终实际引用' : '实际引用待确认',m.state === 'completed' ? '无引用来源' : '草稿尚未确认实际引用');
      sources.append(candidates,actual);
    }
    node.append(sources);
    return {node,text,state, meta(payload) {
      skill = payload.skill_name || ''; updateState('generating');
      CampusMCP.renderCalls(tools,payload.tool_calls);
      showSources(candidates,payload.citations || [],'本轮候选来源（不代表已引用）','无候选来源');
    }, done(payload) {
      skill = payload.skill_name || ''; updateState('completed');
      showSources(actual,payload.citations || [],'最终实际引用','无引用来源');
    }, failure(value) { updateState(value); }};
  }
  async function loadHistory(id, expected) {
    const value = await json('/api/conversations/'+id);
    if (!active || expected !== view || id !== selected) return;
    $('conversation-title').textContent = value.conversation.title || '新会话';
    $('messages').replaceChildren(...value.messages.map(m => message(m).node));
    return value;
  }
  async function select(id) {
    if (compact.matches) disclosure.open = false;
    stop(); selected = id; const expected = ++view; $('source-text').textContent = '';
    $('messages').replaceChildren(); controls();
    await loadHistory(id,expected); await loadList();
  }
  async function loadList() {
    const {conversations} = await json('/api/conversations'); if (!active) return;
    $('conversations').replaceChildren();
    for (const c of conversations) {
      const row = document.createElement('li'), button = document.createElement('button');
      button.type = 'button'; button.textContent = c.title || '新会话'; button.setAttribute('aria-current',String(c.id === selected));
      button.addEventListener('click', () => select(c.id).catch(e => say(e.message))); row.append(button); $('conversations').append(row);
    }
  }
  function fillEditor() {
    const item = agents.find(a => String(a.id) === $('edit-agent').value);
    $('agent-name').value = item?.name || ''; $('prompt').value = item?.prompt || '';
    $('agent-enabled').checked = item?.enabled ?? true;
    for (const input of $('agent-skills').querySelectorAll('input')) input.checked = Boolean(item?.skill_ids?.includes(Number(input.value)));
    for (const input of $('agent-materials').querySelectorAll('input')) input.checked = Boolean(item?.material_ids?.includes(input.value));
  }
  async function loadConfig(teacher) {
    const values = await Promise.all([json('/api/agents'),json('/api/skills'),...(teacher ? [json('/api/materials')] : [])]); if (!active) return;
    agents = values[0].agents; skills = values[1].skills; materials = values[2]?.items || [];
    const option = a => { const node = document.createElement('option'); node.value = a.id; node.textContent = a.name; return node; };
    if (!teacher) { $('agent').replaceChildren(...agents.map(option)); controls(); return; }
    const previous = $('edit-agent').value, fresh = document.createElement('option'); fresh.value = ''; fresh.textContent = '新建助手';
    $('edit-agent').replaceChildren(fresh,...agents.map(option)); $('edit-agent').value = agents.some(a => String(a.id) === previous) ? previous : '';
    const bindings = (target,items,name) => {
      target.replaceChildren();
      if (!items.length) target.textContent = '暂无可绑定项。';
      for (const item of items) { const label = document.createElement('label'), input = document.createElement('input'); input.type = 'checkbox'; input.value = item.id; label.append(input,document.createTextNode(name(item))); target.append(label); }
    };
    bindings($('agent-skills'),skills,item => item.name+(item.enabled ? '' : '（已停用）'));
    bindings($('agent-materials'),materials,item => item.original_name);
    fillEditor();
    $('skills').replaceChildren();
    for (const skill of skills) {
      const label = document.createElement('label'), toggle = document.createElement('input'); toggle.type = 'checkbox'; toggle.checked = skill.enabled;
      label.append(toggle, document.createTextNode(skill.name+'：'+skill.description)); $('skills').append(label);
      label.className = 'skill-row'; toggle.dataset.skillToggle = skill.id;
      const edit = document.createElement('button'); edit.type = 'button'; edit.textContent = '查看 / 编辑'; edit.dataset.skillId = skill.id;
      edit.addEventListener('click', () => editSkill(skill.id).catch(e => say(e.message))); label.append(edit);
      toggle.addEventListener('change', async () => {
        toggle.disabled = true;
        try { await json('/api/skills/'+skill.id,write('PUT',{enabled:toggle.checked})); await loadConfig(true); if (editingSkill === skill.id) await editSkill(skill.id); say('技能状态已保存，下一轮生效。'); }
        catch(e) { toggle.checked = !toggle.checked; say(e.message); }
        finally { toggle.disabled = false; }
      });
    }
  }
  async function editSkill(id) {
    const skill = await json('/api/skills/'+id); if (!active) return;
    editingSkill = skill.id; sourceOriginal = skill.source; $('skill-source').value = skill.source; $('skill-heading').textContent = '编辑技能：'+skill.name; $('delete-skill').hidden = false;
  }
  $('new-skill').addEventListener('click', () => { editingSkill = null; sourceOriginal = null; $('skill-source').value = ''; $('skill-file').value = ''; $('skill-heading').textContent = '新建技能'; $('delete-skill').hidden = true; });
  $('skill-file').addEventListener('change', async () => {
    const file = $('skill-file').files[0]; if (!file) return;
    try { if (file.size > 32000) throw new Error('技能文本最多8000字。'); const text = new TextDecoder('utf-8',{fatal:true}).decode(await file.arrayBuffer()); if ([...text].length > 8000) throw new Error('技能文本最多8000字。'); if (active) { sourceOriginal = text; $('skill-source').value = text; say('已读取文件，请检查后保存技能。'); } }
    catch(e) { say(e.message || '请选择有效的UTF-8文本。'); }
  });
  $('skill-config').addEventListener('submit', async event => {
    event.preventDefault(); const button = $('save-skill'); if (button.disabled) return;
    // textarea normalizes CRLF for display. Preserve the original file/GET
    // bytes when the displayed text has not actually been edited.
    const displayed = $('skill-source').value, source = sourceOriginal !== null && displayed === sourceOriginal.replace(/\r\n?/g,'\n') ? sourceOriginal : displayed;
    if ([...source].length > 8000) { say('技能文本最多8000字。'); return; } button.disabled = true;
    try { const skill = await json('/api/skills'+(editingSkill ? '/'+editingSkill : ''),write(editingSkill ? 'PUT' : 'POST',{source})); await loadConfig(true); await editSkill(skill.id); say('技能已保存，下一轮生效。'); }
    catch(e) { say(e.message); } finally { button.disabled = false; }
  });
  $('delete-skill').addEventListener('click', async () => {
    if (!editingSkill || !confirm('删除该技能并解除当前绑定？历史回答保持原样。')) return;
    const button = $('delete-skill'); button.disabled = true;
    try { const response = await CampusAPI.request('/api/skills/'+editingSkill,{method:'DELETE'}); if (!response.ok) { const value = await response.json(); throw new Error(value.error || '删除失败。'); } $('new-skill').click(); await loadConfig(true); say('技能已删除，当前绑定已解除。'); }
    catch(e) { say(e.message); } finally { button.disabled = false; }
  });
  $('edit-agent').addEventListener('change',fillEditor);
  $('config').addEventListener('submit', async event => {
    event.preventDefault(); const button = event.submitter; if (button.disabled) return; button.disabled = true;
    const id = $('edit-agent').value;
    const value = {name:$('agent-name').value.trim(),prompt:$('prompt').value.trim(),enabled:$('agent-enabled').checked,skill_ids:[...$('agent-skills').querySelectorAll('input:checked')].map(input => Number(input.value)),material_ids:[...$('agent-materials').querySelectorAll('input:checked')].map(input => input.value)};
    try { const saved = await json('/api/agents'+(id ? '/'+id : ''),write(id ? 'PUT' : 'POST',value)); await loadConfig(true); $('edit-agent').value = saved.id; fillEditor(); say('助手已保存，下一轮生效。'); }
    catch(e) { say(e.message); } finally { button.disabled = false; }
  });
  $('create').addEventListener('click', async () => {
    if (busy || $('create').disabled) return; $('create').disabled = true;
    try { const c = await json('/api/conversations',write('POST',{agent_id:Number($('agent').value)})); await select(c.id); say('会话已创建。'); }
    catch(e) { say(e.message); } finally { controls(); }
  });
  $('send').addEventListener('submit', async event => {
    event.preventDefault(); if (busy || !selected) return;
    const content = $('question').value.trim(); if ([...content].length < 2 || [...content].length > 500) { say('请输入2–500字的问题。'); return; }
    busy = true; const id = selected, expected = view, controller = new AbortController(); stream = controller; controls();
    const draft = message({role:'assistant',content:'',state:'generating'});
    let accepted = false, terminal = '';
    say('正在检索本班资料…');
    try {
      const response = await CampusAPI.request('/api/conversations/'+id+'/messages',{...write('POST',{content}),signal:controller.signal});
      if (!response.ok) { const problem = await response.json(); throw new Error(problem.error || '发送失败。'); }
      terminal = await CampusStream.consume(response,(kind,payload) => {
        if (!active || expected !== view) return;
        if (kind === 'meta') { accepted = true; draft.node.dataset.id = payload.message_id; draft.meta(payload); $('messages').append(message({role:'user',content,state:'completed'}).node,draft.node); $('question').value = ''; say('正在回答…'); }
        if (kind === 'delta') draft.text.textContent += payload.text;
        if (kind === 'done') { draft.done(payload); say('回答已保存。'); }
        if (kind === 'error') { draft.failure('failed'); say(payload.error); }
      },controller.signal);
    } catch(e) {
      if (active && expected === view) { draft.failure('interrupted'); say(e.name === 'AbortError' ? '已停止，正在确认保存状态。' : e.message); }
    } finally {
      if (stream === controller) stream = null;
      busy = false; controls();
      if (active && expected === view) {
        try {
          let value = await loadHistory(id,expected);
          for (let retry=0; accepted && value?.messages.at(-1)?.state === 'generating' && retry<3; retry++) {
            await new Promise(resolve => setTimeout(resolve,200)); value = await loadHistory(id,expected);
          }
          // Completion may have committed even when transport lost done.
          if (accepted && !terminal && value?.messages.at(-1)?.state === 'completed') say('已从历史确认回答完成。');
          await loadList();
        } catch(e) { say('历史读取失败，请刷新确认保存状态。'); }
      }
    }
  });
  (async () => {
    if (!CampusAPI.hasToken()) { CampusAPI.clear(); location.replace('/login'); return; }
    const me = await json('/api/auth/me'); if (!active) return;
    const teacher = me.role === 'teacher'; $('identity').textContent = me.class_name+' · '+(teacher ? '教师' : '学生');
    $('teacher').hidden = !teacher; $('student').hidden = teacher;
    if (teacher) CampusMCP.init();
    await loadConfig(teacher); if (!teacher) await loadList();
  })().catch(e => say(e.message));
})();
