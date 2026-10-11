(() => {
  const form = document.getElementById('ask-form');
  const status = document.getElementById('ask-status');
  const answer = document.getElementById('answer');
  const sources = document.getElementById('sources');
  let active = true;
  window.addEventListener('auth-cleared', () => {
    active = false;
    answer.textContent = '';
    sources.replaceChildren();
    document.getElementById('knowledge-scope').textContent = '';
  });
  window.addEventListener('pageshow', event => { if (event.persisted) location.reload(); });
  CampusAPI.request('/api/auth/me').then(async response => {
    if (!response.ok) throw new Error('身份读取失败');
    const me = await response.json();
    if (active) document.getElementById('knowledge-scope').textContent = `${me.class_name} 班 · 仅检索本班材料`;
  }).catch(error => { if (active) status.textContent = error.message; });
  form.addEventListener('submit', async event => {
    event.preventDefault();
    const button = form.querySelector('button');
    button.disabled = true;
    answer.textContent = '';
    sources.replaceChildren();
    status.textContent = '正在检索本班材料…';
    try {
      const response = await CampusAPI.request('/api/ask', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({question: form.elements.question.value}),
      });
      if (!response.ok) throw new Error(response.status === 503 ? '检索服务暂不可用，请检查服务端配置。' :
        response.status === 502 ? '回答或引用校验失败，请重试。' : '提问失败，请检查问题后重试。');
      const data = await response.json();
      if (!active) return;
      answer.textContent = data.answer;
      for (const source of data.sources) {
        const section = document.createElement('section');
        const title = document.createElement('h2');
        title.textContent = `[${source.number}] ${source.title} · 切片 ${source.chunk_index}`;
        const excerpt = document.createElement('pre');
        excerpt.className = 'reading-panel';
        excerpt.textContent = source.chunk_text;
        section.append(title, excerpt);
        sources.append(section);
      }
      status.textContent = data.sources.length ? '回答来源如下' : '';
    } catch (error) { if (active) status.textContent = error.message; }
    finally { button.disabled = false; }
  });
})();
