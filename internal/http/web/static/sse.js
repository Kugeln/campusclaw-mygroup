// Parser shared by the browser and focused Node tests. Never simulates deltas.
((root) => {
  async function consume(response, onEvent, signal) {
    if (!response.ok || !response.headers.get('Content-Type')?.startsWith('text/event-stream') || !response.body) throw new Error('流式响应不可用。');
    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8', {fatal:true});
    let pending = '', event = '', data = [], seenMeta = false, terminal = '';
    function line(value) {
      if (value === '') {
        if (data.length) {
          const payload = JSON.parse(data.join('\n'));
          if (!['meta','delta','done','error'].includes(event) || terminal) throw new Error('流式事件顺序错误。');
          if (event === 'meta') {
            if (seenMeta || typeof payload.message_id !== 'number') throw new Error('流式元信息错误。');
            seenMeta = true;
          } else if (!seenMeta) throw new Error('缺少流式元信息。');
          if (event === 'delta' && typeof payload.text !== 'string') throw new Error('回答片段无效。');
          if (event === 'done' && (payload.state !== 'completed' || !Array.isArray(payload.citations))) throw new Error('回答完成状态无效。');
          if (event === 'error' && typeof payload.error !== 'string') throw new Error('回答错误状态无效。');
          if (event === 'done' || event === 'error') terminal = event;
          onEvent(event, payload);
        }
        event = ''; data = []; return;
      }
      if (value.startsWith(':')) return;
      const split = value.indexOf(':');
      const field = split < 0 ? value : value.slice(0, split);
      let content = split < 0 ? '' : value.slice(split+1);
      if (content.startsWith(' ')) content = content.slice(1);
      if (field === 'event') event = content;
      if (field === 'data') data.push(content);
    }
    function parse(final) {
      let start = 0;
      for (let i=0; i<pending.length; i++) {
        if (pending[i] !== '\r' && pending[i] !== '\n') continue;
        if (pending[i] === '\r' && i === pending.length-1 && !final) break;
        line(pending.slice(start,i));
        if (pending[i] === '\r' && pending[i+1] === '\n') i++;
        start = i+1;
      }
      pending = pending.slice(start);
      if (pending.length > 1048576) throw new Error('回答片段过大。');
    }
    const cancel = () => { reader.cancel().catch(() => {}); };
    signal?.addEventListener('abort', cancel, {once:true});
    try {
      while (!terminal) {
        if (signal?.aborted) throw new DOMException('Canceled','AbortError');
        const {value, done} = await reader.read();
        pending += done ? decoder.decode() : decoder.decode(value, {stream:true});
        parse(done);
        if (done) break;
      }
      if (!terminal) throw new Error('连接中断，正在查询已保存的回答。');
      return terminal;
    } finally {
      signal?.removeEventListener('abort', cancel);
      await reader.cancel().catch(() => {});
      reader.releaseLock();
    }
  }
  const api = Object.freeze({consume});
  if (typeof module !== 'undefined') module.exports = api;
  else root.CampusStream = api;
})(globalThis);
