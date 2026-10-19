const {test} = require('node:test');
const assert = require('node:assert/strict');
const {consume} = require('../../internal/http/web/static/sse.js');
const encoder = new TextEncoder();
const meta = 'event: meta\ndata: {"message_id":1}\n\n';
const done = 'event: done\ndata: {"state":"completed","citations":[]}\n\n';
function response(chunks) {
  return new Response(new ReadableStream({start(c) { for (const chunk of chunks) c.enqueue(chunk); c.close(); }}),{headers:{'Content-Type':'text/event-stream; charset=utf-8'}});
}
test('byte-split UTF-8, CRLF, heartbeat and multiline data',async () => {
  const text=(meta+': heartbeat\n\nevent: delta\ndata: {"text":\ndata: "分析条件"}\n\n'+done).replaceAll('\n','\r\n');
  const seen=[];
  assert.equal(await consume(response([...encoder.encode(text)].map(byte => Uint8Array.of(byte))),(event,data) => seen.push([event,data])),'done');
  assert.deepEqual(seen.map(x=>x[0]),['meta','delta','done']); assert.equal(seen[1][1].text,'分析条件');
});
test('first delta callback runs before remaining content is produced',async () => {
  let release; const gate=new Promise(resolve=>{release=resolve;}); let final=false;
  const stream=new ReadableStream({async start(c) { c.enqueue(encoder.encode(meta+'event: delta\ndata: {"text":"first"}\n\n')); await gate; final=true; c.enqueue(encoder.encode(done)); c.close(); }});
  await consume(new Response(stream,{headers:{'Content-Type':'text/event-stream'}}),(event) => { if(event==='delta') { assert.equal(final,false); release(); } });
});
test('error remains failure, EOF and malformed sequences never complete',async () => {
  assert.equal(await consume(response([encoder.encode(meta+'event: error\ndata: {"error":"failed"}\n\n')]),()=>{}),'error');
  for(const text of [meta+'event: delta\ndata: {"text":"partial"}\n\n',done,meta+meta,meta+'event: delta\ndata: invalid\n\n',meta+'event: done\ndata: {"state":"failed","citations":[]}\n\n']) {
    await assert.rejects(consume(response([encoder.encode(text)]),()=>{}));
  }
});
