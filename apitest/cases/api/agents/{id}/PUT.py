from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=33
def run(c):
    path='/api/agents/'+str(c.agents['a']);before=c.call('GET',path,'ta')
    c.call('PUT',path,'a',c.agent_input(),expected=403);c.call('PUT',path,'tb',c.agent_input(),expected=404)
    c.check(c.call('GET',path,'ta')==before,'forbidden config unchanged')
    changed=c.call('PUT',path,'ta',c.agent_input(prompt='新教师原则：检查每一步',enabled=False))
    c.check(changed['revision']==before['revision']+1,'config revision increments')
    c.call('GET',path,'a',expected=404)
    c.call('PUT',path,'ta',c.agent_input())
if __name__=='__main__':standalone(run)
