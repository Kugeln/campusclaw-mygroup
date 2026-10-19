(() => {
  const $ = id => document.getElementById(id);
  const modes = {hybrid: '综合检索', keyword: '关键词', vector: '语义检索'};
  let active = true, busy = false, sourceView = 0, loadingMaterials = false;
  let materials = [], libraryPage = 0;
  const pageSize = 9;
  const snippets = new ResizeObserver(entries => {
    for (const {target} of entries) {
      const toggle = target.nextElementSibling;
      if (!target.classList.contains('is-expanded')) toggle.hidden = target.scrollHeight <= target.clientHeight + 1;
    }
  });
  const say = text => { if (active) $('notice').textContent = text; };
  const node = (tag, text, className) => {
    const item = document.createElement(tag); item.textContent = text;
    if (className) item.className = className;
    return item;
  };
  async function json(path, options) {
    const response = await CampusAPI.request(path, options);
    if (!response.ok) throw new Error(response.status === 403 ? '权限不足，无法访问该资料。' : response.status === 404 ? '材料不存在或当前不可访问。' : '请求失败，请稍后重试。');
    return response.json();
  }
  function controls() {
    $('search-button').disabled = $('ask-button').disabled = $('mode').disabled = busy;
    $('search-button').textContent = busy ? '处理中…' : '检索';
    $('query').setAttribute('aria-busy', String(busy));
  }
  window.addEventListener('auth-cleared', () => {
    active = false; sourceView++;
    materials = []; libraryPage = 0; snippets.disconnect(); $('library-pagination').hidden = true;
    ['answer','sources','lecture-cards','source-text','source-status','notice','role','result-summary','materials-status'].forEach(id => $(id).replaceChildren());
    document.querySelectorAll('[data-class]').forEach(item => { item.textContent = ''; });
    $('question').value = ''; $('source-panel').hidden = $('results-section').hidden = true;
    document.title = 'CampusClaw';
  });
  window.addEventListener('pageshow', event => { if (event.persisted) location.reload(); });
  $('logout').addEventListener('click', () => CampusAPI.logout());
  $('close-source').addEventListener('click', () => { sourceView++; $('source-panel').hidden = true; $('source-text').textContent = ''; });
  async function readMaterial(id, title, source) {
    const expected = ++sourceView;
    $('source-panel').hidden = false; $('source-title').textContent = title || '材料来源';
    $('source-status').textContent = '正在加载原文…'; $('source-text').textContent = '';
    try {
      const material = await json('/api/materials/' + encodeURIComponent(id));
      if (!active || expected !== sourceView) return;
      $('source-status').textContent = source ? `来源：${source.title} · 切片 ${source.chunk_index}。下方为材料原文。` : '材料原文';
      $('source-text').textContent = material.plain_text;
      $('source-panel').scrollIntoView({block:'start', behavior:'auto'}); $('source-text').focus({preventScroll:true});
    } catch (error) {
      if (active && expected === sourceView) $('source-status').textContent = error.message || '连接失败，请重试。';
    }
  }
  async function loadMaterials() {
    if (loadingMaterials) return;
    loadingMaterials = true; $('reload-materials').disabled = true;
    $('previous-page').disabled = $('next-page').disabled = true;
    $('lecture-cards').setAttribute('aria-busy','true'); $('materials-status').textContent = '正在加载本班讲义…';
    try {
      const {items} = await json('/api/materials'); if (!active) return;
      materials = items; libraryPage = 0; $('lecture-cards').style.minHeight = ''; renderMaterials();
    } catch (error) { if (active) { materials = []; libraryPage = 0; renderMaterials(); $('materials-status').textContent = error.message || '连接失败，请刷新讲义重试。'; } }
    finally { loadingMaterials = false; $('reload-materials').disabled = false; $('lecture-cards').setAttribute('aria-busy','false'); if (active) paginationControls(); }
  }
  function paginationControls() {
    const pages = Math.ceil(materials.length / pageSize);
    $('library-pagination').hidden = pages <= 1;
    $('previous-page').disabled = loadingMaterials || libraryPage === 0;
    $('next-page').disabled = loadingMaterials || libraryPage >= pages - 1;
    $('page-status').textContent = pages ? `第 ${libraryPage + 1} / ${pages} 页 · 共 ${materials.length} 份` : '';
  }
  function renderMaterials() {
      $('lecture-cards').replaceChildren();
      $('lecture-cards').classList.toggle('is-paginated', materials.length > pageSize);
      for (const item of materials.slice(libraryPage * pageSize, (libraryPage + 1) * pageSize)) {
        const card = node('button', '', 'card knowledge-lecture'); card.type = 'button';
        card.dataset.materialId = item.id;
        const extension = item.original_name.split('.').pop().toUpperCase();
        card.append(node('span', extension, 'format-badge'), node('strong', item.original_name, 'knowledge-lecture-title'));
        if (Number.isFinite(item.size)) card.append(node('span', `${item.size.toLocaleString()} 字节`, 'subtitle'));
        if (item.created_at) card.append(node('span', `上传时间（UTC）：${item.created_at}`, 'upload-time'));
        card.append(node('span', '阅读讲义 →', 'knowledge-link'));
        card.addEventListener('click', () => readMaterial(item.id, item.original_name)); $('lecture-cards').append(card);
      }
      // Reserve the measured grid height so the last partial page does not jump.
      if (materials.length > pageSize && window.matchMedia('(min-width: 601px)').matches) {
        $('lecture-cards').style.minHeight = `${$('lecture-cards').getBoundingClientRect().height}px`;
      } else $('lecture-cards').style.minHeight = '';
      $('materials-status').textContent = materials.length ? `共 ${materials.length} 份可访问的讲义` : '本班暂无材料。';
      paginationControls();
  }
  for (const [id, step] of [['previous-page', -1], ['next-page', 1]]) $(id).addEventListener('click', () => {
    if (!active || loadingMaterials) return;
    libraryPage = Math.max(0, Math.min(Math.ceil(materials.length / pageSize) - 1, libraryPage + step));
    renderMaterials();
  });
  function highlighted(text, question) {
    const paragraph = node('p', '', 'knowledge-snippet');
    const terms = [...new Set([question, ...question.split(/\s+/)].filter(Boolean))];
    let offset = 0;
    while (offset < text.length) {
      let first = text.length, term = '';
      for (const candidate of terms) {
        const found = text.indexOf(candidate, offset);
        if (found >= 0 && (found < first || (found === first && candidate.length > term.length))) { first = found; term = candidate; }
      }
      paragraph.append(document.createTextNode(text.slice(offset, first)));
      if (!term) break;
      paragraph.append(node('mark', term)); offset = first + term.length;
    }
    return paragraph;
  }
  $('reload-materials').addEventListener('click', loadMaterials);
  $('query').addEventListener('submit', async event => {
    event.preventDefault(); if (busy || !active) return;
    const question = $('question').value.trim(), mode = $('mode').value;
    if (!question) { say('请输入问题或关键词。'); $('question').focus(); return; }
    const action = event.submitter?.value || 'search';
    busy = true; controls(); say('正在检索本班资料…');
    snippets.disconnect(); $('results-section').hidden = false; $('sources').replaceChildren(); $('answer').textContent = '';
    $('result-summary').textContent = ''; sourceView++; $('source-panel').hidden = true; $('source-text').textContent = '';
    try {
      const value = await json(action === 'search' ? '/api/search?' + new URLSearchParams({q: question, mode}) : '/api/ask', action === 'search' ? {} : {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({question, mode})});
      if (!active) return;
      const sources = value.sources || value.items || [];
      say(''); $('result-summary').textContent = `${modes[mode]} · ${sources.length} 个来源`;
      $('answer').textContent = value.answer || (sources.length ? '' : '资料中未找到相关内容');
      sources.forEach((source, index) => {
        const card = node('article', '', 'card knowledge-result');
        const heading = node('div', '', 'knowledge-result-heading');
        // Search order supplies display numbers; ask supplies citation numbers.
        const number = source.number ?? index + 1;
        heading.append(node('span', `[${number}]`, 'format-badge'), node('h3', source.title));
        const meta = [`来源：${source.title}`, `切片 ${source.chunk_index}`, modes[mode]];
        if (Number.isFinite(source.score)) meta.push(`检索分数 ${source.score.toPrecision(4)}`);
        const button = node('button', `查看来源 [${number}]`); button.type = 'button';
        button.addEventListener('click', () => readMaterial(source.material_id, source.title, source));
        const snippet = highlighted(source.chunk_text, question); snippet.id = `snippet-${index}`;
        const toggle = node('button', '展开全文', 'snippet-toggle'); toggle.type = 'button'; toggle.hidden = true;
        toggle.setAttribute('aria-controls', snippet.id); toggle.setAttribute('aria-expanded', 'false');
        toggle.addEventListener('click', () => {
          const expanded = snippet.classList.toggle('is-expanded');
          toggle.textContent = expanded ? '收起' : '展开全文'; toggle.setAttribute('aria-expanded', String(expanded));
        });
        card.append(heading, node('p', meta.join(' · '), 'subtitle'), snippet, toggle, button);
        $('sources').append(card);
        snippets.observe(snippet);
      });
    } catch (error) { say(error.message || '连接失败，请重试。'); }
    finally { busy = false; controls(); }
  });
  (async () => {
    if (!CampusAPI.hasToken()) { CampusAPI.clear(); location.replace('/login'); return; }
    try {
      const me = await json('/api/auth/me'); if (!active) return;
      const label = me.class_name.endsWith('班') ? me.class_name : me.class_name + '班';
      document.querySelectorAll('[data-class]').forEach(item => { item.textContent = label; });
      $('role').textContent = me.role === 'teacher' ? '教师' : '学生';
      document.title = `${label} · 讲义与检索 · CampusClaw`;
      await loadMaterials();
    } catch (error) { say(error.message || '连接失败，请刷新页面重试。'); $('materials-status').textContent = '暂时无法加载本班讲义。'; $('lecture-cards').setAttribute('aria-busy','false'); }
  })();
})();
