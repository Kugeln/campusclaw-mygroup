from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=31
def run(c):
    before=c.sql('SELECT COUNT(*) AS n FROM assistants')[0]['n']
    c.call('POST','/api/agents','a',c.agent_input(),expected=403)
    c.call('POST','/api/agents',value=c.agent_input(),expected=401)
    c.call('POST','/api/agents','ta',c.agent_input(),expected=400)
    c.call('POST','/api/agents','ta',c.agent_input(name=c.namespace+'-bad',prompt='长'*2001),expected=400)
    c.call('POST','/api/agents','ta',c.agent_input(name=c.namespace+'-foreign',material_ids=[c.materials['b']]),expected=404)
    c.check(c.sql('SELECT COUNT(*) AS n FROM assistants')[0]['n']==before,'rejected assistant writes have no rows')
    value=c.call('POST','/api/agents','ta',c.agent_input(name=c.namespace+'-extra',prompt='',skill_ids=[],material_ids=[]),expected=201)
    row=c.sql('SELECT class_id,prompt,revision,material_ids FROM assistants WHERE id=%s',(value['id'],))[0]
    c.check(row['class_id']==1 and row['prompt']==value['prompt'] and row['revision']==value['revision'],'assistant config persisted with trusted class')
if __name__=='__main__':standalone(run)
