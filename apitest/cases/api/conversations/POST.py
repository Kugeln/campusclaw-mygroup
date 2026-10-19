from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=41
def run(c):
    before=c.sql('SELECT COUNT(*) AS n FROM conversations')[0]['n']
    c.call('POST','/api/conversations','ta',{'agent_id':c.agents['a']},expected=403)
    c.call('POST','/api/conversations','b',{'agent_id':c.agents['a']},expected=404)
    c.call('POST','/api/conversations','a',{'agent_id':c.agents['a'],'class_id':2},expected=400)
    c.call('POST','/api/conversations','a',{},expected=400)
    c.check(c.sql('SELECT COUNT(*) AS n FROM conversations')[0]['n']==before,'rejected creates leave no conversation')
    id=c.create();row=c.sql('SELECT user_id,class_id,agent_id FROM conversations WHERE id=%s',(id,))[0]
    c.check(row=={'user_id':c.users['a'],'class_id':1,'agent_id':c.agents['a']},'conversation binds trusted owner/class')
if __name__=='__main__':standalone(run)
