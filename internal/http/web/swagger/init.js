(() => {
  let controller=null;
  const status=document.getElementById('stream-status'), events=document.getElementById('stream-events');
  const candidates=document.getElementById('stream-candidates'),citations=document.getElementById('stream-citations'),answer=document.getElementById('stream-answer'),skill=document.getElementById('stream-skill');
  const tools=document.getElementById('stream-tools'), toolsRegion=document.getElementById('stream-tools-region');
  const showTools=calls=>{tools.textContent='';toolsRegion.hidden=true;if(Array.isArray(calls)&&calls.length){tools.textContent=JSON.stringify(calls.map(({ref,status,text})=>({ref,status,text})),null,2);toolsRegion.hidden=false;}};
  const reset=()=>{showTools();events.textContent='';candidates.textContent='';citations.textContent='尚未确认';answer.textContent='';skill.textContent='';};
  const forget=()=>{ui.authActions.logout(['bearerAuth']);reset();};
  const ui=SwaggerUIBundle({url:'/openapi.json',dom_id:'#swagger-ui',validatorUrl:null,persistAuthorization:false,displayRequestDuration:true,
    requestInterceptor:request => {
      const url=new URL(request.url,location.origin);
      if(url.origin!==location.origin || url.username || url.password) throw new Error('仅允许同源接口。');
      request.credentials='omit'; request.redirect='error'; return request;
    }});
  window.CampusSwagger=ui;
  window.addEventListener('pagehide',()=>controller?.abort());
  document.getElementById('stream-stop').addEventListener('click',()=>controller?.abort());
  document.getElementById('stream-debug').addEventListener('submit',async event=>{
    event.preventDefault();if(controller)return;
    const token=ui.authSelectors.authorized().getIn(['bearerAuth','value']);
    if(!token){status.textContent='请先 Authorize。';return;}
    const id=Number(document.getElementById('conversation-id').value),content=document.getElementById('stream-content').value.trim();
    if(!Number.isSafeInteger(id)||id<1||[...content].length<2||[...content].length>500){status.textContent='参数无效。';return;}
    controller=new AbortController();const current=controller;reset();status.textContent='连接中…';let messageId=null,completed=false,unauthorized=false;
    document.getElementById('stream-send').disabled=true;document.getElementById('stream-stop').hidden=false;
    try{
      const response=await fetch('/api/conversations/'+id+'/messages',{method:'POST',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({content}),credentials:'omit',redirect:'error',signal:current.signal});
      if(response.status===401){unauthorized=true;forget();}
      if(!response.ok){const problem=await response.json();throw new Error(problem.error||'请求失败');}
      const terminal=await CampusStream.consume(response,(kind,data)=>{
        events.textContent+=kind+': '+JSON.stringify(data)+'\n';status.textContent=kind==='done'?'已完成':kind==='error'?'回答失败':'生成中 · 草稿';
        if(kind==='meta'){showTools(data.tool_calls);messageId=data.message_id;candidates.textContent=JSON.stringify(data.citations || [],null,2);skill.textContent=data.skill_name ? '本轮技能：'+data.skill_name : '本轮未选择技能';}
        if(kind==='delta')answer.textContent+=data.text;
        if(kind==='done'){completed=true;citations.textContent=JSON.stringify(data.citations || [],null,2);}
      },current.signal);
      if(terminal==='error')status.textContent='回答失败';
    }catch(error){status.textContent=error.name==='AbortError'?'已中断，请读历史确认状态。':error.message;}
    finally{
      if(!completed && messageId && !unauthorized){
        try{
          for(let attempt=0;attempt<4;attempt++){
            const response=await fetch('/api/conversations/'+id,{headers:{Authorization:'Bearer '+token},credentials:'omit',redirect:'error'});
            if(response.status===401){forget();break;}if(!response.ok)break;
            const history=await response.json(),message=history.messages.find(m=>m.id===messageId);
            if(message && message.state!=='generating'){
              answer.textContent=message.content;showTools(message.configuration_snapshot?.tool_calls);
              if(message.state==='completed'){status.textContent='已从历史确认回答完成。';citations.textContent=JSON.stringify(message.citations || [],null,2);}
              else status.textContent='未完成草稿 · '+message.state;
              break;
            }
            await new Promise(resolve=>setTimeout(resolve,200));
          }
        }catch{status.textContent+='，请查询历史确认。';}
      }
      if(controller===current)controller=null;document.getElementById('stream-send').disabled=false;document.getElementById('stream-stop').hidden=true;
    }
  });
})();
