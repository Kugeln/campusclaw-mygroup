from pathlib import Path
import sys,json
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=43
def run(c):
    id=c.conversations['a'];path=f'/api/conversations/{id}/messages'
    for role in ['a2','b']:c.call('POST',path,role,{'content':'星河问题'},expected=404)
    c.call('POST',path,'a',{'content':'星河问题','prompt':'override'},expected=400)
    c.control(mode='hold_tail');before=c.control();old=c.call('GET','/api/agents/'+str(c.agents['a']),'ta')
    def first(payload,events):
        row=c.assistant(id)
        c.check(row['status']=='generating' and payload['text'] in row['content'],'first delta persisted before delivery')
        c.check(c.control()['completed']==before['completed'],'first delta arrives before upstream completion')
        count=c.sql('SELECT COUNT(*) AS n FROM conversation_messages WHERE conversation_id=%s',(id,))[0]['n']
        c.call('POST',path,'a',{'content':'星河问题'},expected=409)
        c.check(c.sql('SELECT COUNT(*) AS n FROM conversation_messages WHERE conversation_id=%s',(id,))[0]['n']==count,'concurrent409 no second pair')
        c.call('PUT','/api/agents/'+str(c.agents['a']),'ta',c.agent_input(prompt='下一轮教师原则：检查推导'))
        c.call('PUT','/api/skills/'+str(c.skills['a']),'ta',{'enabled':False})
        c.control(release_tail=True)
    events=c.stream(id,'\u661f\u6cb3\u95ee\u9898',first=first)
    kinds=[kind for kind,_ in events]
    c.check(kinds[0]=='meta' and kinds[-1]=='done' and kinds.count('delta')>=2,'meta then incremental deltas then done')
    first_row=c.assistant(id)
    c.check(first_row['status']=='completed' and first_row['prompt_revision']==old['revision'] and first_row['skill_id']==c.skills['a'],'accepted config and actual skill frozen')
    sources=json.loads(first_row['citations']);c.check(sources==events[-1][1]['citations'],'done equals durable sources')
    c.check(events[0][1]['status']=='ok' and events[0][1]['citations'],'early meta has full candidates')
    c.check(''.join(v['text'] for k,v in events if k=='delta')==first_row['content'],'all delta text equals DB')
    for source in sources:c.check(c.sql('SELECT chunk_text FROM knowledge_chunks WHERE id=%s',(source['chunk_id'],))[0]['chunk_text']==source['chunk_text'],'citation matches DB source')
    c.stream(id,'\u661f\u6cb3\u95ee\u9898')
    next_row=c.assistant(id)
    c.check(next_row['prompt_revision']==old['revision']+1 and next_row['skill_name']=='','next turn applies changed prompt and disabled skill')
    c.check(c.sql('SELECT COUNT(DISTINCT turn_id) AS turns,COUNT(*) AS rows_count FROM conversation_messages WHERE conversation_id=%s',(id,))[0]=={'turns':2,'rows_count':4},'distinct durable turns')
    inputs=c.control()['last_messages'];c.check(len(inputs)<=7 and inputs[0]['role']=='system','system plus at most six effective messages')
    # Reindex only this run's material; original completed references stay frozen.
    c.call('POST','/api/materials/'+c.materials['a']+'/reindex','ta',{'size':250,'overlap':25})
    c.check(c.sql('SELECT citations FROM conversation_messages WHERE id=%s',(first_row['id'],))[0]['citations']==first_row['citations'],'reindex preserves history citation snapshot')
    empty=c.create();calls=c.control()['dialogues'];none=c.stream(empty,'\u5b8c\u5168\u65e0\u6b64\u5185\u5bb9')
    c.check(none[0][1]['status']=='ok' and none[0][1]['citations']==[] and none[-1][1]['citations']==[],'zero-hit still generates with empty sources')
    c.check(c.control()['dialogues']==calls+1 and '[1]' not in ''.join(v['text'] for k,v in none if k=='delta'),'zero-hit one model call; fabricated citations cleaned')
    failed=c.create();c.control(mode='error');events=c.stream(failed,'\u661f\u6cb3\u5931\u8d25');c.check(events[-1][0]=='error' and all(k!='done' for k,_ in events),'stream failure error no done');c.wait_state(failed,'failed');c.reset()
    invalid=c.create();c.control(bad_answer=True);events=c.stream(invalid,'\u661f\u6cb3\u5f15\u7528');c.check(events[-1][0]=='error','invalid citation rejected');c.wait_state(invalid,'failed');c.reset()
    disconnected=c.create();c.control(mode='cancel');c.stream(disconnected,'\u661f\u6cb3\u505c\u6b62',disconnect=True);c.check(c.wait_state(disconnected,'interrupted')['content']!='','disconnect durable prefix');c.reset()
    connection=c.create();c.control(dialogue_failure=True);events=c.stream(connection,'\u661f\u6cb3');c.check([k for k,_ in events]==['meta','error'],'connection failure is HTTP200 after meta, not preflight503');c.wait_state(connection,'failed');c.reset()
    preflight=c.create();calls=c.control()['dialogues'];c.control(embedding_failure=True)
    c.call('POST',f'/api/conversations/{preflight}/messages','a',{'content':'\u661f\u6cb3'},expected=503)
    c.check(c.sql('SELECT COUNT(*) AS n FROM conversation_messages WHERE conversation_id=%s',(preflight,))[0]['n']==2 and c.wait_state(preflight,'failed')['content']=='','accepted pair retained failed before meta')
    c.check(c.control()['dialogues']==calls,'retrieval failure zero model calls')
    c.reset();c.call('PUT','/api/skills/'+str(c.skills['a']),'ta',{'enabled':True})
    c.check(c.call('GET',f'/api/conversations/{id}','a')['messages'][1]['content']==first_row['content'],'history still retains original answer')
if __name__=='__main__':standalone(run)
